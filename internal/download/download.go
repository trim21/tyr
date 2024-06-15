package download

import (
	"net/netip"
	"sync"
	stdSync "sync/atomic"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/kelindar/bitmap"
	"github.com/samber/lo"
	"go.uber.org/atomic"

	"tyr/internal/peer"
	"tyr/internal/req"
)

type State uint8

//go:generate stringer -type=State
const Downloading State = 0
const Stopped State = 1
const Uploading State = 2

type Download struct {
	m sync.Mutex

	meta        metainfo.MetaInfo
	info        metainfo.Info
	infoHash    metainfo.Hash
	totalLength int64

	bm         bitmap.Bitmap
	downloaded atomic.Int64
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
	connections []peer.Peer
	// announce response
	peers       []netip.AddrPort
	trackerTier int
	// if this torrent is initialized
	lazyInitialized atomic.Bool
	err             error
}

// TODO global peers limit
func (d *Download) connectToPeers() {

}

func New(m *metainfo.MetaInfo, downloadDir string) *Download {
	info := lo.Must(m.UnmarshalInfo())

	var private bool
	if info.Private != nil {
		private = *info.Private
	}

	d := &Download{
		meta:   *m,
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
