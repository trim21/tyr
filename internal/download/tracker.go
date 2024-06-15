package download

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/alecthomas/atomic"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/trim21/errgo"
	"github.com/zeebo/bencode"

	"tyr/internal/pkg/null"
	"tyr/internal/util"
)

func (d *Download) CouldAnnounce() bool {
	// check announce interval
	if d.announcePending.Load() {
		return false
	}
	return true
}

func (d *Download) AsyncAnnounce(http *resty.Client) {
	d.asyncAnnounce(http)
	d.connectToPeers()
}

func (d *Download) asyncAnnounce(http *resty.Client) {
	d.announcePending.Store(true)
	defer d.announcePending.Store(false)

	for _, tier := range d.trackers {
		r, err := tier.Announce(d, http)
		if err != nil {
			d.m.Lock()
			d.err = err
			d.m.Unlock()
			// TODO: enable announcing to all tier by config
			return
		}
		if len(d.peers) != 0 {
			d.m.Lock()
			d.peers = append(d.peers, r.Peers...)
			d.peers = lo.UniqBy(d.peers, func(item netip.AddrPort) string {
				return item.String()
			})

			d.m.Unlock()
		}
	}
}

type TrackerTier []*Tracker

func (tier TrackerTier) Announce(d *Download, http *resty.Client) (AnnounceResult, error) {
	for _, t := range tier {
		if t.isBackup {
			continue
		}
		if !time.Now().After(t.nextAnnounce.Load()) {
			return AnnounceResult{}, nil
		}

		log.Trace().Hex("info_hash", d.infoHash.Bytes()).Str("url", t.url).Msg("announce to tracker")

		res, err := http.R().SetQueryParams(util.StrMap{
			"info_hash":  d.infoHash.AsString(),
			"peer_id":    d.peerID.AsString(),
			"port":       "47864",
			"compat":     "1",
			"numwant":    "200",
			"uploaded":   strconv.FormatInt(d.uploaded.Load()-d.uploadAtStart, 10),
			"downloaded": strconv.FormatInt(d.downloaded.Load()-d.downloadAtStart, 10),
			"left":       strconv.FormatInt(d.totalLength-d.completed.Load(), 10),
		}).Get(t.url)
		if err != nil {
			t.err = err
			return AnnounceResult{}, err
		}

		var r trackerAnnounceResponse
		err = bencode.DecodeBytes(res.Body(), &r)
		if err != nil {
			log.Debug().Err(err).Msg("failed to decode tracker response")
			t.err = errgo.Wrap(err, "failed to parse torrent announce response")
			return AnnounceResult{}, err
		}

		if r.FailureReason.Set {
			t.err = errors.New(r.FailureReason.Value)
			return AnnounceResult{}, err
		}

		var result = AnnounceResult{
			Interval: 0,
		}

		if r.Interval.Set {
			result.Interval = time.Second * time.Duration(r.Interval.Value)
			next := time.Now().Add(result.Interval)
			t.nextAnnounce.Store(next)
			log.Trace().Hex("info_hash", d.infoHash.Bytes()).Str("url", t.url).Time("next", next).Msg("next announce")
		}

		if r.Peers.Set {
			if r.Peers.Value[0] == 'l' && r.Peers.Value[len(r.Peers.Value)-1] == 'e' {
				result.Peers = parseNonCompatResponse(r.Peers.Value)
				// non compact response
			} else {
				// compact response
				fmt.Println(string(r.Peers.Value))
				var s []byte
				err = bencode.DecodeBytes(r.Peers.Value, &s)
				if err == nil {
					result.Peers = make([]netip.AddrPort, 0, len(s)/6)
					for i := 0; i < len(s); i += 6 {
						addr := netip.AddrFrom4([4]byte(s[i : i+4]))
						port := binary.BigEndian.Uint16(s[i+4:])
						result.Peers = append(result.Peers, netip.AddrPortFrom(addr, port))
					}
				}
			}

			slices.SortFunc(result.Peers, func(a, b netip.AddrPort) int {
				return bytes.Compare(a.Addr().AsSlice(), b.Addr().AsSlice())
			})
		}

		if r.Peers6.Set {
			if r.Peers6.Value[0] == 'l' && r.Peers6.Value[len(r.Peers6.Value)-1] == 'e' {
				// non compact response
				result.Peers6 = parseNonCompatResponse(r.Peers6.Value)
			} else {
				// compact response
				var s []byte
				if bencode.DecodeBytes(r.Peers.Value, &s) == nil {
					result.Peers6 = make([]netip.AddrPort, len(s)/6)
					for i := 0; i < len(s); i += 18 {
						addr := netip.AddrFrom16([16]byte(s[i : i+16]))
						port := binary.BigEndian.Uint16(s[i+16:])
						result.Peers6 = append(result.Peers6, netip.AddrPortFrom(addr, port))
					}
				}
			}
		}

		return result, nil
	}

	return AnnounceResult{}, nil
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
	Interval time.Duration
	Peers    []netip.AddrPort
	Peers6   []netip.AddrPort
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
	url              string
	peers            []byte
	peers6           []byte
	lastAnnounce     bool
	lastAnnounceTime time.Time
	interval         time.Duration
	nextAnnounce     atomic.Value[time.Time]
	isBackup         bool
	err              error
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
