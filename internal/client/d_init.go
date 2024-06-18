package client

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/dustin/go-humanize"
	"github.com/trim21/errgo"

	"github.com/valyala/bytebufferpool"

	"tyr/internal/pkg/fallocate"
)

type existingFile struct {
	index int
	size  int64
}

func (d *Download) initCheck() error {
	d.log.Debug().Msg("initCheck download task")
	err := os.MkdirAll(d.basePath, os.ModePerm)
	if err != nil {
		return err
	}

	var efs = make(map[int]*existingFile, len(d.info.Files)+1)
	for i, f := range d.info.UpvertedV1Files() {
		p := f.DisplayPath(&d.info)
		f, err := tryAllocFile(i, filepath.Join(d.basePath, p), f.Length)
		if err != nil {
			return err
		}
		if f != nil {
			efs[i] = f
		}
	}

	h := d.buildPieceToCheck(efs)
	if len(h) == 0 {
		return nil
	}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	if cap(buf.B) <= int(d.info.PieceLength) {
		buf.B = make([]byte, d.info.PieceLength)
	}

	var fileSeekCache = make(map[int]int64, len(d.info.Files))
	var fileOpenCache = make(map[int]*os.File, len(d.info.Files))
	defer func() {
		for _, file := range fileOpenCache {
			_ = file.Close()
		}
	}()

	for _, index := range h {
		buf.Reset()
		//d.log.Trace().Msgf("should check piece %d %s", index, humanize.IBytes(uint64(d.ioDown.Status().CurRate)))
		piece := d.pieceInfo[index]
		for _, chunk := range piece.fileChunks {
			f, ok := fileOpenCache[chunk.fileIndex]
			fp := d.filePath(chunk.fileIndex)
			if !ok {
				f, err = os.Open(fp)
				if err != nil {
					return errgo.Wrap(err, fmt.Sprintf("failed to open file %s", fp))
				}
				fileOpenCache[chunk.fileIndex] = f
			}

			if fileSeekCache[chunk.fileIndex] != chunk.offsetOfFile {
				_, err = f.Seek(chunk.offsetOfFile, io.SeekStart)
				if err != nil {
					return errgo.Wrap(err, fmt.Sprintf("failed to read file %s", fp))
				}
			}

			_, err = d.ioDown.IO(io.ReadFull(f, buf.B[len(buf.B):len(buf.B)+int(chunk.length)]))
			if err != nil {
				return errgo.Wrap(err, fmt.Sprintf("failed to read file %s", fp))
			}

			fileSeekCache[chunk.fileIndex] = chunk.offsetOfFile + chunk.length
		}

		sum := sha1.Sum(buf.B[:d.info.PieceLength])
		if bytes.Equal(sum[:], piece.hash) {
			d.bm.Set(index)
		}
	}

	d.log.Debug().Msgf("done size %s", humanize.IBytes(uint64(d.bm.Count())*uint64(d.info.PieceLength)))

	//d.m.Lock()
	d.state = Downloading
	//d.m.Unlock()
	//d.cond.Broadcast()

	//d.Start()

	return nil
}

// update progress by bitmap
func (d *Download) updateProgress() {
	d.completed.Store(int64(d.numPieces) * int64(d.bm.Count()))
}

func (d *Download) filePath(i int) string {
	return filepath.Join(d.basePath, d.info.UpvertedFiles()[i].DisplayPath(&d.info))
}

func (d *Download) buildPieceToCheck(efs map[int]*existingFile) []uint32 {
	if len(efs) == 0 {
		return nil
	}

	var r = make([]uint32, 0, d.info.NumPieces())

	for i := uint32(0); i < d.numPieces; i++ {
		p := d.pieceInfo[i]
		shouldCheck := true
		for _, c := range p.fileChunks {
			ef, ok := efs[c.fileIndex]
			if !ok {
				shouldCheck = false
				break
			}

			if c.offsetOfFile > ef.size || c.offsetOfFile+c.length > ef.size {
				shouldCheck = false
				break
			}
		}

		if shouldCheck {
			r = append(r, i)
		}
	}

	return r
}

type pieceInfo struct {
	hash       []byte
	fileChunks []pieceInfoFileChunk
}

func buildPieceInfos(info metainfo.Info) []pieceInfo {
	p := make([]pieceInfo, info.NumPieces())

	for i := 0; i < info.NumPieces(); i++ {
		p[i] = getPieceInfo(i, info)
	}

	return p
}

func getPieceInfo(i int, info metainfo.Info) pieceInfo {
	return pieceInfo{
		hash:       info.Pieces[i : i+20],
		fileChunks: pieceFileInfos(i, info),
	}
}

type pieceInfoFileChunk struct {
	fileIndex    int
	offsetOfFile int64
	length       int64
}

func pieceFileInfos(i int, info metainfo.Info) []pieceInfoFileChunk {
	var pieceStart = int64(i) * info.PieceLength

	if len(info.Files) == 0 {
		return []pieceInfoFileChunk{{
			offsetOfFile: pieceStart,
			length:       min(info.Length-info.PieceLength*int64(i), info.PieceLength),
		}}
	}

	var currentFileStart int64 = 0
	var needToRead = info.PieceLength
	var fileIndex = 0

	var result []pieceInfoFileChunk

	for needToRead > 0 {
		f := info.Files[fileIndex]
		currentFileEnd := currentFileStart + f.Length
		currentReadStart := pieceStart + (info.PieceLength - needToRead)
		if currentFileStart <= currentReadStart && currentReadStart <= currentFileEnd {
			result = append(result, pieceInfoFileChunk{
				fileIndex:    fileIndex,
				offsetOfFile: currentReadStart - currentFileStart,
				length:       needToRead,
			})
		}

		needToRead = needToRead - min(currentFileEnd-currentReadStart, needToRead)
	}

	if needToRead < 0 {
		panic("unexpected need to read")
	}

	return result
}

func tryAllocFile(index int, path string, size int64) (*existingFile, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return nil, fallocate.Fallocate(f, 0, size)
	}

	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var ef *existingFile
	fs := stat.Size()
	if fs != 0 {
		ef = &existingFile{index: index, size: fs}
	}
	if fs != size {
		return nil, errgo.Wrap(fallocate.Fallocate(f, fs, size-fs), "failed to alloc file")
	}

	return ef, nil
}
