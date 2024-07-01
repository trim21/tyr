package core

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/negrel/assert"
	"github.com/trim21/errgo"

	"tyr/internal/meta"
	"tyr/internal/pkg/fallocate"
	"tyr/internal/pkg/gfs"
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
	for i, tf := range d.info.Files {
		p := tf.Path
		f, e := tryAllocFile(i, filepath.Join(d.basePath, p), tf.Length, d.c.Config.App.Fallocate.Load())
		if e != nil {
			return e
		}
		if f != nil {
			efs[i] = f
		}
	}

	h := d.buildPieceToCheck(efs)
	if len(h) == 0 {
		return nil
	}

	var fileSeekCache = make(map[int]int64, len(d.info.Files))
	var fileOpenCaches = make(map[int]*os.File, len(d.info.Files))
	defer func() {
		for _, file := range fileOpenCaches {
			_ = file.Close()
		}
	}()

	//bucket := ratelimit.NewBucketWithQuantum(time.Second/10, units.MiB*500, units.MiB*50)

	// `len(buf.B)` is always 0, and `cap(buf.B)` is always PieceLength
	// we use `size` to keep trace length of `buf.B`
	var size int64
	for _, index := range h {
		size = 0
		piece := d.pieceInfo[index]
		sum := sha1.New()
		for _, chunk := range piece.fileChunks {
			f, ok := fileOpenCaches[chunk.fileIndex]
			fp := filepath.Join(d.basePath, d.info.Files[chunk.fileIndex].Path)
			if !ok {
				f, err = os.Open(fp)
				if err != nil {
					return errgo.Wrap(err, fmt.Sprintf("failed to open file %s", fp))
				}
				fileOpenCaches[chunk.fileIndex] = f
			}

			if fileSeekCache[chunk.fileIndex] != chunk.offsetOfFile {
				_, err = f.Seek(chunk.offsetOfFile, io.SeekStart)
				if err != nil {
					return errgo.Wrap(err, fmt.Sprintf("failed to read file %s", fp))
				}
			}

			//_, err = d.ioDown.IO(io.ReadFull(f, buf.B[size:size+chunk.length]))
			_, err = d.ioDown.IO64(io.CopyN(sum, gfs.NewReader(d.ctx, f), chunk.length))
			if err != nil {
				return errgo.Wrap(err, fmt.Sprintf("failed to read file %s", fp))
			}
			size += chunk.length

			//bucket.Wait(chunk.length)

			fileSeekCache[chunk.fileIndex] = chunk.offsetOfFile + chunk.length
		}

		if [sha1.Size]byte(sum.Sum(nil)) == d.info.Pieces[index] {
			d.bm.Set(index)
		}
	}

	return nil
}

func (d *Download) buildPieceToCheck(efs map[int]*existingFile) []uint32 {
	if len(efs) == 0 {
		return nil
	}

	var r = make([]uint32, 0, d.info.NumPieces)

	for i := uint32(0); i < d.info.NumPieces; i++ {
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

type pieceFileChunks struct {
	fileChunks []pieceInfoFileChunk
}

func buildPieceInfos(info meta.Info) []pieceFileChunks {
	p := make([]pieceFileChunks, info.NumPieces)

	for i := uint32(0); i < info.NumPieces; i++ {
		p[i] = getPieceInfo(i, info)
	}

	return p
}

func getPieceInfo(i uint32, info meta.Info) pieceFileChunks {
	assert.False(info.Pieces[i] == [20]byte{})

	return pieceFileChunks{
		fileChunks: pieceFileInfos(i, info),
	}
}

type pieceInfoFileChunk struct {
	fileIndex    int
	offsetOfFile int64
	length       int64
}

func pieceFileInfos(i uint32, info meta.Info) []pieceInfoFileChunk {
	var pieceStart = int64(i) * info.PieceLength
	var currentFileStart int64 = 0
	var needToRead = info.PieceLength
	var fileIndex = 0

	var result []pieceInfoFileChunk

	for needToRead > 0 {
		f := info.Files[fileIndex]
		currentFileEnd := currentFileStart + f.Length
		currentReadStart := pieceStart + (info.PieceLength - needToRead)

		if currentFileStart <= currentReadStart && currentReadStart <= currentFileEnd {

			shouldRead := min(currentFileEnd-currentReadStart, needToRead)

			result = append(result, pieceInfoFileChunk{
				fileIndex:    fileIndex,
				offsetOfFile: currentReadStart - currentFileStart,
				length:       shouldRead,
			})

			needToRead = needToRead - shouldRead
		}

		currentFileStart += f.Length

		fileIndex++

		if fileIndex >= len(info.Files) {
			break
		}
	}

	if needToRead < 0 {
		panic("unexpected need to read")
	}

	return result
}

func tryAllocFile(index int, path string, size int64, doAlloc bool) (*existingFile, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		if !doAlloc {
			return nil, nil
		}

		f, err = os.Create(path)
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

	if doAlloc {
		if fs != size {
			return nil, errgo.Wrap(fallocate.Fallocate(f, fs, size-fs), "failed to alloc file")
		}
	}

	return ef, nil
}
