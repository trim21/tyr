package download

import (
	"sync"

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
	meta       metainfo.MetaInfo
	info       metainfo.Info
	bm         bitmap.Bitmap
	downloaded atomic.Int64
	uploaded   atomic.Int64
	completed  atomic.Int64

	peerID peer.ID

	uploadAtStart   atomic.Int64
	downloadAtStart atomic.Int64

	resChan chan req.Response

	m           sync.Mutex
	downloadDir string
	state       uint8
	private     bool
	trackers    []TrackerTier
	peers       []peer.Peer
	trackerTier int
	// if this torrent is initialized
	lazyInitialized atomic.Bool
}

func New(m *metainfo.MetaInfo, downloadDir string) *Download {
	info := lo.Must(m.UnmarshalInfo())

	d := &Download{
		meta:   *m,
		peerID: peer.NewID(),
		// already validated
		info: info,
		// there maybe 1 uint64 extra data here.
		bm:          make(bitmap.Bitmap, info.PieceLength/8+8),
		private:     *info.Private,
		downloadDir: downloadDir,
	}

	d.setAnnounceList(m)

	return d
}
