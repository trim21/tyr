package client

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	stdSync "sync/atomic"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/types/infohash"
	"github.com/dustin/go-humanize"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/atomic"

	"tyr/internal/meta"
	"tyr/internal/pkg/bm"
	"tyr/internal/pkg/flowrate"
	"tyr/internal/pkg/global"
	"tyr/internal/proto"
)

type State uint8

//go:generate stringer -type=State
const Downloading State = 0
const Stopped State = 1
const Uploading State = 2
const Checking State = 3
const Moving State = 3
const Error State = 4

type peerRequest struct {
	peer *Peer
	req  proto.ChunkRequest
}

// Download manage a download task
// ctx should be canceled when torrent is removed, not stopped.
type Download struct {
	meta              metainfo.MetaInfo
	log               zerolog.Logger
	ctx               context.Context
	err               error
	reqHistory        *xsync.MapOf[uint32, downloadReq]
	cancel            context.CancelFunc
	cond              *sync.Cond
	c                 *Client
	ioDown            *flowrate.Monitor
	ioUp              *flowrate.Monitor
	ResChan           chan proto.ChunkResponse
	conn              *xsync.MapOf[netip.AddrPort, *Peer]
	connectionHistory *xsync.MapOf[netip.AddrPort, connHistory]
	bm                *bm.Bitmap

	pdMutex     sync.RWMutex
	pieceData   map[uint32][]*proto.ChunkResponse
	pieceChunks [][]proto.ChunkRequest

	basePath        string
	key             string
	downloadDir     string
	tags            []string
	pieceInfo       []pieceFileChunks
	trackers        []TrackerTier
	peers           []netip.AddrPort
	AddAt           int64
	CompletedAt     atomic.Int64
	info            meta.Info
	downloaded      atomic.Int64
	corrupted       atomic.Int64
	done            atomic.Bool
	uploaded        atomic.Int64
	completed       atomic.Int64
	checkProgress   atomic.Int64
	uploadAtStart   int64
	downloadAtStart int64
	lazyInitialized atomic.Bool
	seq             atomic.Bool
	m               sync.RWMutex
	peersMutex      sync.RWMutex
	connMutex       sync.RWMutex
	announcePending stdSync.Bool
	peerID          PeerID
	state           State
	private         bool
}

type pieceChunk struct {
	data   []byte
	offset uint32
}

func (c *Client) NewDownload(m *metainfo.MetaInfo, info meta.Info, basePath string, tags []string) *Download {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Download{
		ctx:      ctx,
		info:     info,
		cancel:   cancel,
		meta:     *m,
		c:        c,
		log:      log.With().Stringer("info_hash", info.Hash).Logger(),
		state:    Checking,
		peerID:   NewPeerID(),
		tags:     tags,
		basePath: basePath,

		reqHistory: xsync.NewMapOf[uint32, downloadReq](),

		AddAt: time.Now().Unix(),

		ResChan: make(chan proto.ChunkResponse, 1),

		ioDown: flowrate.New(time.Second, time.Second),
		ioUp:   flowrate.New(time.Second, time.Second),

		conn:              xsync.NewMapOf[netip.AddrPort, *Peer](),
		connectionHistory: xsync.NewMapOf[netip.AddrPort, connHistory](),

		peers: []netip.AddrPort{
			netip.MustParseAddrPort("192.168.1.3:50025"),
		},

		pieceInfo: buildPieceInfos(info),

		pieceData:   make(map[uint32][]*proto.ChunkResponse, 20),
		pieceChunks: buildPieceChunk(info),

		//key:
		// there maybe 1 uint64 extra data here.
		bm: bm.New(info.NumPieces),

		downloadDir: basePath,
	}

	d.seq.Store(true)
	d.cond = sync.NewCond(&d.m)

	if !global.Dev {
		d.setAnnounceList(m)
	}

	//spew.Dump(d.pieceChunks[0])
	//spew.Dump(d.pieceChunks[len(d.pieceChunks)-1])

	return d
}

func (d *Download) Move(target string) error {
	return errors.New("not implemented")
}

func (d *Download) Display() string {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	d.m.RLock()
	defer d.m.RUnlock()

	_, _ = fmt.Fprintf(buf, "%s | %.20s | %.2f%% | %s | %s | %d ↓",
		d.state, d.info.Name,
		float64(int64(d.bm.Count())*d.info.PieceLength)/float64(d.info.TotalLength)*100.0,
		humanize.IBytes(uint64(d.downloaded.Load())),
		d.ioDown.Status().RateString(), d.conn.Size())

	for _, tier := range d.trackers {
		for _, t := range tier.trackers {
			t.RLock()
			fmt.Fprintf(buf, " ( %d %s )", t.peerCount, t.url)
			if t.err != nil {
				_, _ = fmt.Fprintf(buf, " | %s", t.err)
			}
			t.RUnlock()
		}
	}

	var s []peerDisplay

	d.conn.Range(func(key netip.AddrPort, p *Peer) bool {
		s = append(s, peerDisplay{
			Up:     humanize.IBytes(uint64(p.ioOut.Status().CurRate)),
			Down:   humanize.IBytes(uint64(p.ioIn.Status().CurRate)),
			Client: p.UserAgent.Load(),
			Addr:   key,
		})

		return true
	})

	sort.Slice(s, func(i, j int) bool {
		return s[i].Addr.Compare(s[j].Addr) < 1
	})

	for _, p := range s {
		if p.Client == nil {
			_, _ = fmt.Fprintf(buf, "\n ↓ %6s/s | ↑ %6s/s | %s", p.Down, p.Up, p.Addr)
		} else {
			_, _ = fmt.Fprintf(buf, "\n ↓ %6s/s | ↑ %6s/s | %s | %s", p.Down, p.Up, *p.Client, p.Addr)
		}
	}

	return buf.String()
}

type peerDisplay struct {
	Up     string
	Down   string
	Client *string
	Addr   netip.AddrPort
}

// if download encounter an error must stop downloading/uploading
func (d *Download) setError(err error) {
	d.m.Lock()
	d.err = err
	d.state = Error
	d.m.Unlock()
}

func canonicalName(info metainfo.Info, infoHash infohash.T) string {
	// yes, there are some torrent have this name
	name := info.Name
	if (info.NameUtf8) != "" {
		name = info.NameUtf8
	}

	if name == "" {
		return infoHash.HexString()
	}

	if len(info.Files) != 0 {
		return name
	}
	s := strings.Split(name, ".")
	if len(s) == 0 {
		return name
	}

	return strings.Join(s[:len(s)-1], ".")
}

type connHistory struct {
	lastTry   time.Time
	err       error
	timeout   bool
	connected bool
}
