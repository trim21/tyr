package client

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo/v4"

	"tyr/internal/meta"
	"tyr/internal/pkg/bufpool"
	"tyr/internal/pkg/global/tasks"
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

	d.log.Debug().Msgf("done size %s", humanize.IBytes(uint64(d.bm.Count())*uint64(d.info.PieceLength)))

	if d.bm.Count() == d.info.NumPieces {
		d.state = Uploading
	} else {
		d.state = Downloading
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

func (d *Download) have(index uint32) {
	d.conn.Range(func(addr netip.AddrPort, p *Peer) bool {
		tasks.Submit(func() {
			p.Have(index)
		})
		return true
	})
}

func (d *Download) backgroundResHandler() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case res := <-d.ResChan:
			d.handleRes(res)
		}
	}
}

func (d *Download) handleRes(res proto.ChunkResponse) {
	d.log.Trace().
		Any("res", echo.Map{
			"piece":  res.PieceIndex,
			"offset": res.Begin,
			"length": len(res.Data),
		}).Msg("res received")

	d.ioDown.Update(len(res.Data))
	d.downloaded.Add(int64(len(res.Data)))

	d.pdMutex.Lock()
	defer d.pdMutex.Unlock()

	chunks, ok := d.pieceData[res.PieceIndex]
	if !ok {
		chunks = make([]*proto.ChunkResponse, len(d.pieceChunks[res.PieceIndex]))
	}

	pi := res.Begin / defaultBlockSize
	chunks[pi] = &res

	filled := true
	for _, res := range chunks {
		if res == nil {
			filled = false
			break
		}
	}

	if filled {
		piece := bufpool.Get()
		for _, chunk := range chunks {
			piece.Write(chunk.Data)
		}

		h := sha1.Sum(piece.B)

		if h != d.info.Pieces[res.PieceIndex] {
			d.corrupted.Add(d.info.PieceLength)
			fmt.Println("data mismatch", res.PieceIndex)
			bufpool.Put(piece)
			d.pieceData[res.PieceIndex] = nil
			return
		}

		tasks.Submit(func() {
			defer bufpool.Put(piece)
			pieces := d.pieceInfo[res.PieceIndex]
			var offset int64 = 0
			for _, chunk := range pieces.fileChunks {
				f, err := os.OpenFile(filepath.Join(d.basePath, d.info.Files[chunk.fileIndex].Path), os.O_RDWR|os.O_CREATE, os.ModePerm)
				if err != nil {
					d.setError(err)
					return
				}
				defer f.Close()

				_, err = f.Seek(chunk.offsetOfFile, io.SeekStart)
				if err != nil {
					d.setError(err)
					return
				}

				_, err = f.Write(piece.B[offset:chunk.length])
				if err != nil {
					d.setError(err)
					return
				}

				offset += chunk.length
			}

			d.pdMutex.Lock()
			delete(d.pieceData, res.PieceIndex)
			d.pdMutex.Unlock()

			d.bm.Set(res.PieceIndex)

			d.log.Trace().Msgf("piece %d done", res.PieceIndex)
			d.have(res.PieceIndex)

			if d.bm.Count() == d.info.NumPieces {
				d.m.Lock()
				d.state = Uploading
				d.m.Unlock()
			}
		})
	}

	d.pieceData[res.PieceIndex] = chunks
}

func (d *Download) backgroundPieceHandle() {
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
			if d.ctx.Err() != nil {
				return
			}
			d.m.Lock()
			if d.state == Uploading {
				d.cond.Wait()
			}
			d.m.Unlock()

			//d.log.Debug().Msg("backgroundPieceHandle")

			//weight := avaPool.Get()

			//if cap(weight) < int(d.numPieces) {
			//}
			//var h heap.Interface[pair.Pair[uint32, uint32]]
			//weight := make([]pair.Pair[uint32, uint32], 0, int(d.numPieces))

			d.log.Trace().Msgf("connections %d", d.conn.Size())

			if d.seq.Load() {
				d.scheduleSeq()
				continue
			}

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

			for i, priority := range h {
				if i > 5 {
					break
				}

				fmt.Println(priority.Index, priority.Weight)
			}
		}
		//fmt.Println("index", v.Index)
		//avaPool.Put(weight)
	}
}

type downloadReq struct {
	conn *Peer
	r    proto.ChunkRequest
}

func buildPieceChunk(info meta.Info) [][]proto.ChunkRequest {
	var result = make([][]proto.ChunkRequest, 0, (info.PieceLength+defaultBlockSize-1)/defaultBlockSize*int64(info.NumPieces))

	var numPerPiece = (info.PieceLength + defaultBlockSize - 1) / defaultBlockSize

	for i := uint32(0); i < info.NumPieces; i++ {
		var rr = make([]proto.ChunkRequest, 0, numPerPiece)

		pieceStart := int64(i) * info.PieceLength

		pieceLen := min(info.PieceLength, info.TotalLength-pieceStart)

		for n := int64(0); n < numPerPiece; n++ {
			begin := defaultBlockSize * int64(n)
			length := uint32(min(pieceLen-begin, defaultBlockSize))

			if length <= 0 {
				break
			}

			rr = append(rr, proto.ChunkRequest{
				PieceIndex: i,
				Begin:      uint32(begin),
				Length:     length,
			})
		}

		if len(rr) == 0 {
			break
		}

		result = append(result, rr)
	}

	return result
}

func (d *Download) scheduleSeq() {
	var found = 0
	for pi, chunks := range d.pieceChunks {
		index := uint32(pi)

		if d.bm.Get(index) {
			continue
		}

		d.conn.Range(func(addr netip.AddrPort, p *Peer) bool {
			if !p.Bitmap.Get(index) {
				return true
			}

			for _, chunk := range chunks {
				p.Request(chunk)
			}

			found++

			return false
		})

		if found > 20 {
			break
		}
	}
}
