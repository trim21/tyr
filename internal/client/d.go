package client

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	stdSync "sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/mxk/go-flowrate/flowrate"
	"github.com/negrel/assert"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"tyr/internal/peer"
	"tyr/internal/pkg/bm"
	"tyr/internal/req"
)

type State uint8

//go:generate stringer -type=State
const Downloading State = 0
const Stopped State = 1
const Uploading State = 2
const Checking State = 3
const Moving State = 3
const Error State = 4

type Download struct {
	ctx               context.Context
	cancel            context.CancelFunc
	info              metainfo.Info
	meta              metainfo.MetaInfo
	log               zerolog.Logger
	err               error
	cond              *sync.Cond
	c                 *Client
	ioDown            *flowrate.Monitor
	ioUp              *flowrate.Monitor
	resChan           chan req.Response
	conn              *xsync.MapOf[string, *peer.Peer]
	connectionHistory *xsync.MapOf[string, connHistory]
	basePath          string
	key               string
	downloadDir       string
	tags              []string
	pieceInfo         []pieceInfo
	trackers          []TrackerTier
	peers             []netip.AddrPort
	bm                bm.Bitmap
	totalLength       int64
	downloaded        atomic.Int64
	done              atomic.Bool
	uploaded          atomic.Int64
	completed         atomic.Int64
	checkProgress     atomic.Int64
	uploadAtStart     int64
	downloadAtStart   int64
	trackerTier       int
	lazyInitialized   atomic.Bool
	m                 sync.RWMutex
	peersMutex        sync.RWMutex
	connMutex         sync.RWMutex
	numPieces         uint32
	announcePending   stdSync.Bool
	infoHash          metainfo.Hash
	peerID            peer.ID
	state             State
	private           bool
}

func (c *Client) NewDownload(m *metainfo.MetaInfo, info metainfo.Info, basePath string, tags []string) *Download {
	var private bool
	if info.Private != nil {
		private = *info.Private
	}

	ctx, cancel := context.WithCancel(context.Background())

	var infoHash = m.HashInfoBytes()
	d := &Download{
		ctx:      ctx,
		cancel:   cancel,
		meta:     *m,
		c:        c,
		log:      log.With().Hex("info_hash", infoHash.Bytes()).Logger(),
		state:    Checking,
		peerID:   peer.NewID(),
		tags:     tags,
		basePath: basePath,

		ioDown: flowrate.New(time.Second, time.Second),
		ioUp:   flowrate.New(time.Second, time.Second),

		totalLength:       info.TotalLength(),
		info:              info,
		infoHash:          infoHash,
		conn:              xsync.NewMapOf[string, *peer.Peer](),
		connectionHistory: xsync.NewMapOf[string, connHistory](),

		pieceInfo: buildPieceInfos(info),
		numPieces: uint32(info.NumPieces()),

		//key:
		// there maybe 1 uint64 extra data here.
		bm:          bm.New(int(info.PieceLength/8 + 8)),
		private:     private,
		downloadDir: basePath,
	}

	d.cond = sync.NewCond(&d.m)

	assert.Equal(uint32(len(d.pieceInfo)), d.numPieces)

	d.setAnnounceList(m)

	return d
}

func (d *Download) Move(target string) error {
	return errors.New("not implemented")
}

func (d *Download) Display() string {
	d.m.RLock()
	defer d.m.RUnlock()
	return fmt.Sprintf("%s | %.20s | %.2f%%", d.state, d.info.Name, float64(d.completed.Load())/float64(d.totalLength)*100.0)
}

// if download encounter an error must stop downloading/uploading
func (d *Download) setError(err error) {
	d.m.Lock()
	d.err = err
	d.state = Error
	d.m.Unlock()
}

func canonicalName(info metainfo.Info, infoHash torrent.InfoHash) string {
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
	connected bool
}
