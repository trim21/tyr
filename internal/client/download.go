package client

import (
	"net"
	"net/netip"
	"sync"
	stdSync "sync/atomic"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/kelindar/bitmap"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"go.uber.org/atomic"

	"tyr/internal/peer"
	"tyr/internal/pkg/global"
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
	// announce response

	peersMutex sync.RWMutex
	peers      []netip.AddrPort

	connMutex sync.RWMutex
	conn      *xsync.MapOf[string, *peer.Peer]

	connectionHistory *xsync.MapOf[string, connHistory]

	trackerTier int
	// if this torrent is initialized
	lazyInitialized atomic.Bool
	err             error
}

func (d *Download) cleanDeadPeers() {
	d.conn.Range(func(key string, value *peer.Peer) bool {
		if value.Dead() {
			d.conn.Delete(key)
			d.c.sem.Release(1)
		}
		return true
	})
}

func (d *Download) connectToPeers() {
	d.cleanDeadPeers()

	d.peersMutex.RLock()
	if len(d.peers) > 0 {
		log.Trace().Msg("connectToPeers")
	}

	for _, p := range d.peers {
		a := p.String()
		log.Trace().Msgf("check peer %s", a)
		_, connected := d.conn.Load(a)

		if connected {
			log.Trace().Msgf("peer connected")
			continue
		}

		h, ok := d.connectionHistory.Load(a)
		if ok {
			if h.lastTry.After(time.Now().Add(-time.Minute)) {
				continue
			}
		}

		if !d.c.sem.TryAcquire(1) {
			break
		}

		conn, err := global.Dialer.Dial("tcp", a)
		if err != nil {
			d.connectionHistory.Store(a, connHistory{lastTry: time.Now()})
			continue
		}
		d.connectionHistory.Store(a, connHistory{lastTry: time.Now(), connected: true})

		log.Trace().Msgf("connected to peer %s", a)
		d.conn.Store(a, d.peerFromConn(conn, a))
	}

	d.peersMutex.RUnlock()

	d.peersMutex.Lock()
	d.peers = d.peers[:0]
	d.peersMutex.Unlock()
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
		info:              info,
		infoHash:          m.HashInfoBytes(),
		conn:              xsync.NewMapOf[string, *peer.Peer](),
		connectionHistory: xsync.NewMapOf[string, connHistory](),
		//key:
		// there maybe 1 uint64 extra data here.
		bm:          make(bitmap.Bitmap, info.PieceLength/8+8),
		private:     private,
		downloadDir: downloadDir,
	}

	d.setAnnounceList(m)

	return d
}

type connHistory struct {
	lastTry   time.Time
	connected bool
}
