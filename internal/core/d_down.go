package core

import (
	"crypto/sha1"
	"fmt"
	"net/netip"
	"sort"
	"time"

	"github.com/docker/go-units"

	"tyr/internal/pkg/global/tasks"
	"tyr/internal/pkg/mempool"
	"tyr/internal/proto"
)

type downloadReq struct {
	conn *Peer
	r    proto.ChunkRequest
}

func (d *Download) have(index uint32) {
	d.conn.Range(func(addr netip.AddrPort, p *Peer) bool {
		tasks.Submit(func() {
			p.Have(index)
		})
		return true
	})
}

func (d *Download) handleRes(res proto.ChunkResponse) {
	d.log.Trace().
		Any("res", map[string]any{
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
		chunks = make([]*proto.ChunkResponse, (d.pieceLength(res.PieceIndex)+defaultBlockSize-1)/defaultBlockSize)
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
		go func() {
			err := d.writePieceToDisk(res.PieceIndex, chunks)
			if err != nil {
				d.setError(err)
			}
		}()

		delete(d.pieceData, res.PieceIndex)
		return
	}

	d.pieceData[res.PieceIndex] = chunks
}

func (d *Download) writePieceToDisk(pieceIndex uint32, chunks []*proto.ChunkResponse) error {
	buf := mempool.Get()

	for _, chunk := range chunks {
		buf.Write(chunk.Data)
	}

	h := sha1.Sum(buf.B)
	if h != d.info.Pieces[pieceIndex] {
		d.corrupted.Add(d.info.PieceLength)
		fmt.Println("data mismatch", pieceIndex)
		mempool.Put(buf)
		return nil
	}

	tasks.Submit(func() {
		defer mempool.Put(buf)
		pieces := d.pieceInfo[pieceIndex]
		var offset int64 = 0

		for _, chunk := range pieces.fileChunks {
			f, err := d.openFileWithCache(chunk.fileIndex)
			if err != nil {
				d.setError(err)
				return
			}
			defer f.Release()

			_, err = f.File.WriteAt(buf.B[offset:offset+chunk.length], chunk.offsetOfFile)
			if err != nil {
				d.setError(err)
				return
			}

			offset += chunk.length
		}

		d.pdMutex.Lock()
		delete(d.pieceData, pieceIndex)
		d.pdMutex.Unlock()

		d.bm.Set(pieceIndex)

		d.log.Trace().Msgf("buf %d done", pieceIndex)
		d.have(pieceIndex)

		if d.bm.Count() == d.info.NumPieces {
			d.m.Lock()
			d.state = Uploading
			d.ioDown.Reset()
			d.m.Unlock()
		}
	})

	return nil
}

func (d *Download) backgroundReqHandle() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if d.ctx.Err() != nil {
				return
			}
			d.m.Lock()
			if d.state == Uploading {
				d.cond.Wait()
			}
			d.m.Unlock()

			//d.log.Debug().Msg("backgroundReqHandle")

			//weight := avaPool.Get()

			//if cap(weight) < int(d.numPieces) {
			//}
			//var h heap.Interface[pair.Pair[uint32, uint32]]
			//weight := make([]pair.Pair[uint32, uint32], 0, int(d.numPieces))

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

func (d *Download) scheduleSeq() {
	var found int64 = 0

	for index := uint32(0); index < d.info.NumPieces; index++ {
		//for pi, chunks := range d.pieceChunks {
		//	index := uint32(pi)
		if d.bm.Get(index) {
			continue
		}

		chunks := pieceChunks(d.info, index)

		send := false
		d.conn.Range(func(addr netip.AddrPort, p *Peer) bool {
			if !p.Bitmap.Get(index) {
				return true
			}

			for _, chunk := range chunks {
				p.Request(chunk)
			}

			send = true
			return false
		})

		if send {
			found++
		}

		if found*d.info.PieceLength >= units.GiB {
			break
		}
	}
}
