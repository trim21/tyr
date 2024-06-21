package client

import (
	"fmt"
	"net/netip"
	"sort"
	"time"

	"github.com/docker/go-units"

	"tyr/internal/proto"
)

const defaultBlockSize = units.KiB * 16

func (d *Download) Start() {
	d.m.Lock()
	if d.done.Load() {
		d.state = Uploading
	} else {
		d.state = Downloading
	}
	d.m.Unlock()
	d.cond.Broadcast()
}

func (d *Download) Stop() {
	d.m.Lock()
	d.state = Stopped
	d.m.Unlock()
	d.cond.Broadcast()
}

func (d *Download) Check() {
	d.m.Lock()
	d.state = Checking
	d.bm.Clear()
	d.m.Unlock()
	d.cond.Broadcast()
}

// Init check existing files
func (d *Download) Init() {
	d.log.Debug().Msg("initializing download")

	err := d.initCheck()
	if err != nil {
		d.setError(err)
		d.log.Err(err).Msg("failed to initCheck torrent data")
	}

	go d.startBackground()
}

func (d *Download) startBackground() {
	d.log.Trace().Msg("start goroutine")

	go d.backgroundPieceHandle()
	go d.backgroundResHandler()

	go func() {
		for {
			if d.ctx.Err() != nil {
				return
			}

			d.m.Lock()
			if d.state == Stopped {
				d.log.Trace().Msg("paused, waiting")
				d.cond.Wait()
			}
			d.m.Unlock()

			d.connectToPeers()

			time.Sleep(time.Second * 20)
		}
	}()

	for {
		if d.ctx.Err() != nil {
			return
		}
		d.m.Lock()
		if d.state == Stopped {
			d.log.Trace().Msg("paused, waiting")
			d.cond.Wait()
		}
		d.m.Unlock()

		d.TryAnnounce()

		time.Sleep(time.Second * 5)
	}
}

type Priority struct {
	Index  uint32
	Weight uint32
}

type PriorityQueue []Priority

func (p *PriorityQueue) Len() int {
	return len(*p)
}

func (p *PriorityQueue) Less(i, j int) bool {
	return (*p)[i].Weight > (*p)[j].Weight
}

func (p *PriorityQueue) Swap(i, j int) {
	(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
}

func (p *PriorityQueue) Push(item Priority) {
	*p = append(*p, item)
}

func (p *PriorityQueue) Pop() Priority {
	old := *p
	n := len(old)
	x := old[n-1]
	*p = old[:n-1]
	return x
}

func (d *Download) backgroundResHandler() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case res := <-d.ResChan:
			_ = res
		}
	}
}

func (d *Download) backgroundPieceHandle() {
	for {
		time.Sleep(time.Second * 5)

		if d.ctx.Err() != nil {
			return
		}
		d.m.Lock()
		if d.state == Uploading {
			d.cond.Wait()
		}
		d.m.Unlock()

		d.log.Debug().Msg("backgroundPieceHandle")

		//weight := avaPool.Get()

		//if cap(weight) < int(d.numPieces) {
		//}
		//var h heap.Interface[pair.Pair[uint32, uint32]]
		//weight := make([]pair.Pair[uint32, uint32], 0, int(d.numPieces))

		d.log.Trace().Msgf("connections %d", d.conn.Size())

		h := make(PriorityQueue, d.info.NumPieces)

		for i := range h {
			h[i].Index = uint32(i)
		}

		d.conn.Range(func(key netip.AddrPort, p *Peer) bool {
			if p.Bitmap.Count() == 0 {
				return true
			}

			p.Bitmap.Range(func(i uint32) {
				h[i].Weight++
			})

			return true
		})

		sort.Sort(&h)

		if d.seq.Load() {
			d.scheduleSeq()
			continue
		}

		for i, priority := range h {
			if i > 5 {
				break
			}

			fmt.Println(priority.Index, priority.Weight)
		}

		//fmt.Println("index", v.Index)
		//avaPool.Put(weight)
	}
}

type peerRes struct {
	addr netip.AddrPort
	res  proto.ChunkResponse
}

type downloadReq struct {
	conn *Peer
	r    proto.ChunkRequest
}

func (d *Download) scheduleSeq() {
	for i := uint32(0); i < d.info.NumPieces; i++ {
		if d.bm.Get(i) {
			continue
		}

		d.conn.Range(func(key netip.AddrPort, p *Peer) bool {
			if p.Choked.Load() {
				return true
			}

			if !p.Bitmap.Get(i) {
				return true
			}

			p.Bitmap.RangeX(func(pi uint32) bool {
				if pi < i {
					return true
				}

				// have piece, just send
				r := proto.ChunkRequest{
					PieceIndex: i,
					Begin:      0,
					Length:     defaultBlockSize,
				}

				d.reqHistory.Store(i, downloadReq{
					p,
					r,
				})

				p.reqChan <- r

				return false
			})

			return true
		})
	}
}
