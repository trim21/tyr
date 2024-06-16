package client

import (
	"net"
	"net/netip"
	"sync"
	stdSync "sync/atomic"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/kelindar/bitmap"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"go.uber.org/atomic"

	"tyr/global"
	"tyr/internal/peer"
	"tyr/internal/req"
)

type State uint8

//go:generate stringer -type=State
const Downloading State = 0
const Stopped State = 1
const Uploading State = 2

type Download struct {
	m sync.RWMutex

	c *Client

	meta        metainfo.MetaInfo
	info        metainfo.Info
	infoHash    metainfo.Hash
	totalLength int64

	bm         bitmap.Bitmap
	downloaded atomic.Int64
	done       atomic.Bool
	uploaded   atomic.Int64
	completed  atomic.Int64

	peerID peer.ID

	uploadAtStart   int64
	downloadAtStart int64

	resChan chan req.Response

	announcePending stdSync.Bool

	key string

	downloadDir string
	state       uint8
	private     bool
	trackers    []TrackerTier
	connections []*peer.Peer
	// announce response
	peers       []netip.AddrPort
	trackerTier int
	// if this torrent is initialized
	lazyInitialized atomic.Bool
	err             error
}

func (d *Download) cleanDeadPeers() {
	d.m.Lock()
	before := len(d.connections)
	d.connections = lo.Filter(d.connections, func(item *peer.Peer, index int) bool {
		return !item.Dead()
	})
	after := len(d.connections)
	d.m.Unlock()

	d.c.sem.Release(int64(after - before))
}

func (d *Download) connectToPeers() {
	log.Trace().Msg("connectToPeers")
	d.cleanDeadPeers()

	for _, p := range d.peers {
		a := p.String()
		log.Trace().Msgf("check peer %s", a)
		d.m.RLock()
		connected := lo.ContainsBy(d.connections, func(item *peer.Peer) bool {
			return item.Address == a
		})
		d.m.RUnlock()

		if connected {
			log.Trace().Msgf("peer connected")
			continue
		}

		if !d.c.sem.TryAcquire(1) {
			return
		}

		conn, err := global.Dialer.Dial("tcp", a)
		if err != nil {
			continue
		}
		log.Trace().Msgf("connected to peer %s", a)
		d.m.Lock()
		d.connections = append(d.connections, d.peerFromConn(conn, a))
		d.m.Unlock()
	}
}

func (d *Download) peerFromConn(conn net.Conn, a string) *peer.Peer {
	return peer.New(conn, d.infoHash, uint32(d.info.NumPieces()), a)
}

func (c *Client) NewDownload(m *metainfo.MetaInfo, downloadDir string) *Download {
	info := lo.Must(m.UnmarshalInfo())

	var private bool
	if info.Private != nil {
		private = *info.Private
	}

	d := &Download{
		meta:   *m,
		c:      c,
		peerID: peer.NewID(),
		// already validated
		info:     info,
		infoHash: m.HashInfoBytes(),
		//key:
		// there maybe 1 uint64 extra data here.
		bm:          make(bitmap.Bitmap, info.PieceLength/8+8),
		private:     private,
		downloadDir: downloadDir,
	}

	d.setAnnounceList(m)

	return d
}
