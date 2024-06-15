package download

import (
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type AnnounceResult struct {
}

func (d *Download) CouldAnnounce() bool {
	// check announce interval
	if d.announcePending.Load() {
		return false
	}
	return true
}

func (d *Download) AsyncAnnounce(http *resty.Client) {
	d.announcePending.Store(true)
	defer d.announcePending.Store(false)

	log.Trace().Hex("info_hash", d.infoHash.Bytes()).Msg("announce")
	for _, tier := range d.trackers {
		tier.Announce(d)
	}
	time.Sleep(time.Second * 10)
}

type TrackerTier []*Tracker

func (t TrackerTier) Announce(d *Download) (AnnounceResult, error) {
	return AnnounceResult{}, nil
}

type Tracker struct {
	url          string
	peers        []byte
	peers6       []byte
	lastAnnounce bool
	isBackup     bool
}

func (d *Download) setAnnounceList(t *metainfo.MetaInfo) {
	if len(t.UpvertedAnnounceList()) == 0 {
		return
	}

	for _, tier := range t.UpvertedAnnounceList() {
		d.trackers = append(d.trackers, lo.Map(tier, func(item string, index int) *Tracker {
			return &Tracker{url: item}
		}))
	}
}
