package download

import (
	"github.com/anacrolix/torrent/metainfo"
	"github.com/samber/lo"
)

type AnnounceResult struct {
}

func (d *Download) Announce() (AnnounceResult, error) {
	return d.trackers[d.trackerTier].Announce(d)
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
