package client

import (
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
	m sync.RWMutex

	log zerolog.Logger

	c *Client

	// a "good" name for directory
	basePath string

	tags []string

	meta        metainfo.MetaInfo
	info        metainfo.Info
	infoHash    metainfo.Hash
	totalLength int64

	ioDown *flowrate.Monitor
	ioUp   *flowrate.Monitor

	bm         bm.Bitmap
	downloaded atomic.Int64
	done       atomic.Bool
	uploaded   atomic.Int64
	completed  atomic.Int64

	checkProgress atomic.Int64

	peerID peer.ID

	pieceInfo []pieceInfo
	numPieces uint32

	uploadAtStart   int64
	downloadAtStart int64

	resChan chan req.Response

	announcePending stdSync.Bool

	key string

	downloadDir string
	state       State
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

func (c *Client) NewDownload(m *metainfo.MetaInfo, info metainfo.Info, basePath string, tags []string) *Download {
	var private bool
	if info.Private != nil {
		private = *info.Private
	}

	var infoHash = m.HashInfoBytes()
	d := &Download{
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

	assert.Equal(uint32(len(d.pieceInfo)), d.numPieces)

	d.setAnnounceList(m)

	return d
}
func (d *Download) Start() {
	d.m.Lock()
	if d.done.Load() {
		d.state = Uploading
	} else {
		d.state = Downloading
	}
	d.m.Unlock()
}

func (d *Download) Stop() {
	d.m.Lock()
	d.state = Stopped
	d.m.Unlock()
}

func (d *Download) Move(target string) error {
	return errors.New("not implemented")
}

func (d *Download) Display() string {
	d.m.RLock()
	d.m.RUnlock()
	return fmt.Sprintf("%.20s | %.2f%%", d.info.Name, float64(d.completed.Load())/float64(d.totalLength)*100.0)
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
