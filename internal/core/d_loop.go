package core

import (
	"crypto/sha1"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog/log"

	"tyr/internal/meta"
	"tyr/internal/pkg/as"
	"tyr/internal/pkg/bufpool"
	"tyr/internal/pkg/filepool"
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

	d.m.Lock()
	d.state = Checking
	d.m.Unlock()

	err := d.initCheck()
	if err != nil {
		d.setError(err)
		d.log.Err(err).Msg("failed to initCheck torrent data")
	}

	d.ioDown.Reset()

	d.log.Debug().Msgf("done size %s", humanize.IBytes(uint64(d.bm.Count())*uint64(d.info.PieceLength)))

	d.m.Lock()
	if d.bm.Count() == d.info.NumPieces {
		d.state = Uploading
	} else {
		d.state = Downloading
	}
	d.m.Unlock()

	go d.startBackground()
}

func (d *Download) startBackground() {
	d.log.Trace().Msg("start goroutine")

	go d.backgroundReqHandle()
	go d.backgroundResHandler()

	go func() {
		for {
			if d.ctx.Err() != nil {
				return
			}

			d.m.Lock()

		LOOP:
			for {
				switch d.state {
				case Uploading, Downloading:
					break LOOP
				case Stopped, Moving, Checking, Error:
					d.cond.Wait()
				}
			}

			d.m.Unlock()

			d.connectToPeers()

			time.Sleep(time.Second)
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

func (d *Download) giveBackFileCache(f *fileOpenCache) {
	d.fileOpenMutex.L.Lock()
	defer d.fileOpenMutex.L.Unlock()

	f.borrowed = false
	d.fileOpenCache[f.index] = f

	d.fileOpenMutex.Broadcast()
}

func (d *Download) openFileWithCache(fileIndex int) (*filepool.File, error) {
	p := filepath.Join(d.basePath, d.info.Files[fileIndex].Path)
	return filepool.Open(p, os.O_RDWR|os.O_CREATE, os.ModePerm, time.Hour)
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
	buf := bufpool.Get()

	for _, chunk := range chunks {
		buf.Write(chunk.Data)
	}

	h := sha1.Sum(buf.B)
	if h != d.info.Pieces[pieceIndex] {
		d.corrupted.Add(d.info.PieceLength)
		fmt.Println("data mismatch", pieceIndex)
		bufpool.Put(buf)
		return nil
	}

	tasks.Submit(func() {
		defer bufpool.Put(buf)
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

func (d *Download) readPiece(index uint32) ([]byte, error) {
	pieces := d.pieceInfo[index]
	var buf = make([]byte, d.pieceLength(index))

	var offset int64 = 0
	for _, chunk := range pieces.fileChunks {
		f, err := d.openFileWithCache(chunk.fileIndex)
		if err != nil {
			return nil, err
		}

		_, err = f.File.ReadAt(buf[offset:offset+chunk.length], chunk.offsetOfFile)
		if err != nil {
			f.Release()
			return nil, err
		}

		offset += chunk.length
		f.Release()
	}

	return buf, nil
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

type downloadReq struct {
	conn *Peer
	r    proto.ChunkRequest
}

func pieceChunk(pieceIndex uint32, info meta.Info) []proto.ChunkRequest {
	var numPerPiece = (info.PieceLength + defaultBlockSize - 1) / defaultBlockSize

	var rr = make([]proto.ChunkRequest, 0, numPerPiece)

	pieceStart := int64(pieceIndex) * info.PieceLength

	pieceLen := min(info.PieceLength, info.TotalLength-pieceStart)

	for n := int64(0); n < numPerPiece; n++ {
		begin := defaultBlockSize * int64(n)
		if pieceLen-begin < 0 {
			break
		}
		length := as.Uint32(min(pieceLen-begin, defaultBlockSize))

		if length <= 0 {
			break
		}

		rr = append(rr, proto.ChunkRequest{
			PieceIndex: pieceIndex,
			Begin:      uint32(begin),
			Length:     length,
		})
	}

	return rr
}

// TODO: use too much memory
func buildPieceChunk(info meta.Info) [][]proto.ChunkRequest {
	log.Debug().Stringer("info_hash", info.Hash).Msg("build piece chunk")
	var result = make([][]proto.ChunkRequest, 0, (info.PieceLength+defaultBlockSize-1)/defaultBlockSize*int64(info.NumPieces))

	for i := uint32(0); i < info.NumPieces; i++ {
		result = append(result, pieceChunk(i, info))
	}

	if len(result) == 0 {
		panic("unexpected result")
	}

	fmt.Println(humanize.IBytes(uint64(nestedSizeOf(reflect.ValueOf(result)))))

	return result
}

func nestedSizeOf(rv reflect.Value) uintptr {
	var sum uintptr = 0
	switch rv.Type().Kind() {
	case reflect.Chan, reflect.Array, reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		sum += rv.Type().Size()
	case reflect.Struct:
		//sum += rv.Type().Size()
		for i := 0; i < rv.NumField(); i++ {
			sum += nestedSizeOf(rv.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			sum += nestedSizeOf(rv.Index(i))
		}
	case reflect.Ptr:
		sum += nestedSizeOf(rv.Elem())
	case reflect.String:
		sum += uintptr(rv.Len())
	case reflect.Interface:
		sum += nestedSizeOf(rv.Elem())
	case reflect.Map:
		for _, k := range rv.MapKeys() {
			sum += nestedSizeOf(k)
			sum += nestedSizeOf(rv.MapIndex(k))
		}
	}

	return sum
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
