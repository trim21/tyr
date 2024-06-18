package client

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/trim21/errgo"
	"github.com/valyala/bytebufferpool"
	"github.com/zeebo/bencode"

	"tyr/internal/pkg/null"
)

const EventStarted = "started"
const EventCompleted = "completed"
const EventStopped = "stopped"

func (d *Download) TryAnnounce() {
	if !d.announcePending.Load() {
		d.AsyncAnnounce()
		return
	}
}

func (d *Download) AsyncAnnounce() {
	d.asyncAnnounce()
	d.connectToPeers()
}

// TODO: enable announcing to all tier by config
func (d *Download) asyncAnnounce() {
	d.announcePending.Store(true)
	defer d.announcePending.Store(false)

	// TODO: all level tracker announce
	for _, tier := range d.trackers {
		r, err := tier.Announce(d)
		if err != nil {
			continue
		}
		if len(r.Peers) != 0 {
			d.peersMutex.Lock()
			d.peers = append(d.peers, r.Peers...)
			d.peers = lo.UniqBy(d.peers, func(item netip.AddrPort) string {
				return item.String()
			})
			d.peersMutex.Unlock()
		}
	}
}

type TrackerTier struct {
	trackers []*Tracker
}

func (tier TrackerTier) Announce(d *Download) (AnnounceResult, error) {
	return AnnounceResult{}, nil
}

func (tier TrackerTier) announce(d *Download, event string) (AnnounceResult, error) {
	now := time.Now()
	for _, t := range tier.trackers {
		if !now.After(t.nextAnnounce) {
			return AnnounceResult{}, nil
		}

		r, err := t.announce(d, event)
		if err != nil {
			t.Lock()
			t.err = err
			t.Unlock()
			continue
		}

		if r.FailedReason.Set {
			t.Lock()
			t.err = errors.New(r.FailedReason.Value)
			t.Unlock()
			return AnnounceResult{}, nil
		}

		r.Peers = lo.Uniq(r.Peers)

		return r, nil
	}

	return AnnounceResult{}, nil
}

func (tier TrackerTier) announceStop(d *Download) {
	for _, t := range tier.trackers {
		err := t.announceStop(d)
		if err != nil {
			// nothing we can actually to handle this
			return
		}
	}

	return
}

type nonCompactAnnounceResponse struct {
	IP   string `bencode:"ip"`
	Port uint16 `bencode:"port"`
}

func parseNonCompatResponse(data []byte) []netip.AddrPort {
	var s []nonCompactAnnounceResponse
	if err := bencode.DecodeBytes(data, &s); err != nil {
		return nil
	}

	var results = make([]netip.AddrPort, 0, len(s))
	for _, item := range s {
		a, err := netip.ParseAddr(item.IP)
		if err != nil {
			continue
		}
		results = append(results, netip.AddrPortFrom(a, item.Port))
	}

	return results
}

type AnnounceResult struct {
	FailedReason null.String
	Peers        []netip.AddrPort
	Interval     time.Duration
}

type trackerAnnounceResponse struct {
	FailureReason null.Null[string]             `bencode:"failure reason"`
	Peers         null.Null[bencode.RawMessage] `bencode:"peers"`
	Peers6        null.Null[bencode.RawMessage] `bencode:"peers6"`
	Interval      null.Null[int]                `bencode:"interval"`
	Complete      null.Null[int]                `bencode:"complete"`
	Incomplete    null.Null[int]                `bencode:"incomplete"`
}

type Tracker struct {
	sync.RWMutex
	lastAnnounceTime time.Time
	nextAnnounce     time.Time
	err              error
	url              string
}

func (t *Tracker) announce(d *Download, event string) (AnnounceResult, error) {
	d.log.Trace().Str("url", t.url).Msg("announce to tracker")

	res, err := d.c.http.R().
		SetQueryParam("info_hash", d.infoHash.AsString()).
		SetQueryParam("peer_id", d.peerID.AsString()).
		SetQueryParam("port", strconv.FormatUint(uint64(d.c.Config.App.P2PPort), 10)).
		SetQueryParam("compat", "1").
		SetQueryParam("event", event).
		SetQueryParam("uploaded", strconv.FormatInt(d.uploaded.Load()-d.uploadAtStart, 10)).
		SetQueryParam("downloaded", strconv.FormatInt(d.downloaded.Load()-d.downloadAtStart, 10)).
		SetQueryParam("left", strconv.FormatInt(d.totalLength-d.completed.Load(), 10)).
		Get(t.url)
	if err != nil {
		return AnnounceResult{}, errgo.Wrap(err, "failed to connect to tracker")
	}

	var r trackerAnnounceResponse
	err = bencode.DecodeBytes(res.Body(), &r)
	if err != nil {
		log.Debug().Err(err).Str("res", res.String()).Msg("failed to decode tracker response")
		return AnnounceResult{}, errgo.Wrap(err, "failed to parse torrent announce response")
	}

	if r.FailureReason.Set {
		return AnnounceResult{FailedReason: r.FailureReason}, nil
	}

	var result = AnnounceResult{
		Interval: time.Minute * 30,
	}

	if r.Interval.Set {
		result.Interval = time.Second * time.Duration(r.Interval.Value)
	}

	t.nextAnnounce = time.Now().Add(result.Interval)
	d.log.Trace().Str("url", t.url).Time("next", t.nextAnnounce).Msg("next announce")

	// BEP says we must support both format
	if r.Peers.Set {
		if r.Peers.Value[0] == 'l' && r.Peers.Value[len(r.Peers.Value)-1] == 'e' {
			result.Peers = parseNonCompatResponse(r.Peers.Value)
			// non compact response
		} else {
			// compact response
			var b = bytebufferpool.Get()
			b.Reset()
			err = bencode.DecodeBytes(r.Peers.Value, &b.B)
			if err == nil {
				result.Peers = make([]netip.AddrPort, 0, len(b.B)/6)
				for i := 0; i < len(b.B); i += 6 {
					addr := netip.AddrFrom4([4]byte(b.B[i : i+4]))
					port := binary.BigEndian.Uint16(b.B[i+4:])
					result.Peers = append(result.Peers, netip.AddrPortFrom(addr, port))
				}
			}
			bytebufferpool.Put(b)
		}

		slices.SortFunc(result.Peers, func(a, b netip.AddrPort) int {
			return bytes.Compare(a.Addr().AsSlice(), b.Addr().AsSlice())
		})
	}

	if r.Peers6.Set {
		if r.Peers6.Value[0] == 'l' && r.Peers6.Value[len(r.Peers6.Value)-1] == 'e' {
			// non compact response
			result.Peers = append(result.Peers, parseNonCompatResponse(r.Peers6.Value)...)
		} else {
			// compact response
			var b = bytebufferpool.Get()
			b.Reset()
			if bencode.DecodeBytes(r.Peers.Value, &b.B) == nil {
				for i := 0; i < b.Len(); i += 18 {
					addr := netip.AddrFrom16([16]byte(b.B[i : i+16]))
					port := binary.BigEndian.Uint16(b.B[i+16:])
					result.Peers = append(result.Peers, netip.AddrPortFrom(addr, port))
				}
			}
			bytebufferpool.Put(b)
		}
	}

	result.Peers = lo.Uniq(result.Peers)

	return result, nil
}

func (t *Tracker) announceStop(d *Download) error {
	d.log.Trace().Str("url", t.url).Msg("announce to tracker")

	_, err := d.c.http.R().
		SetQueryParam("info_hash", d.infoHash.AsString()).
		SetQueryParam("peer_id", d.peerID.AsString()).
		SetQueryParam("port", strconv.FormatUint(uint64(d.c.Config.App.P2PPort), 10)).
		SetQueryParam("compat", "1").
		SetQueryParam("event", EventStopped).
		SetQueryParam("uploaded", strconv.FormatInt(d.uploaded.Load()-d.uploadAtStart, 10)).
		SetQueryParam("downloaded", strconv.FormatInt(d.downloaded.Load()-d.downloadAtStart, 10)).
		SetQueryParam("left", strconv.FormatInt(d.totalLength-d.completed.Load(), 10)).
		Get(t.url)
	if err != nil {
		return errgo.Wrap(err, "failed to parse torrent announce response")
	}

	return nil
}

func (d *Download) setAnnounceList(m *metainfo.MetaInfo) {
	for _, tier := range m.UpvertedAnnounceList() {
		t := TrackerTier{trackers: lo.Map(lo.Shuffle(tier), func(item string, index int) *Tracker {
			return &Tracker{url: item, nextAnnounce: time.Now()}
		})}

		d.trackers = append(d.trackers, t)
	}
}
