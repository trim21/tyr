//go:build gc

package gfs

import (
	"fmt"
	"io"

	"github.com/colega/zeropool"
	"github.com/docker/go-units"

	"tyr/internal/pkg/mempool"
)

var pool = zeropool.New(func() []byte {
	return make([]byte, units.MiB*4)
})

func CopyReaderAt(dst io.Writer, ra io.ReaderAt, offset int64, size int64) (int64, error) {
	if size >= units.MiB*4 {
		return copyReaderAtBuffed(dst, ra, offset, size)
	}

	// read and write it in one shot.

	buf := mempool.Get()
	defer mempool.Put(buf)

	if cap(buf.B) < int(size) {
		buf.B = make([]byte, size)
	}

	n, err := ra.ReadAt(buf.B[:size], offset)

	if err != nil {
		return int64(n), err
	}

	n, err = dst.Write(buf.B[:size])

	return int64(n), nil
}

// copyReaderAtBuffed copies to a writer from a given reader at for the given number of bytes.
// from https://github.com/containerd/containerd/blob/v1.7.18/content/helpers.go#L289
func copyReaderAtBuffed(dst io.Writer, ra io.ReaderAt, offset, size int64) (int64, error) {
	copied, err := copyWithBuffer(dst, io.NewSectionReader(ra, offset, size))
	if err != nil {
		return copied, fmt.Errorf("failed to copy: %w", err)
	}
	if copied < size {
		// Short writes would return its own error, this indicates a read failure
		return copied, fmt.Errorf("failed to read expected number of bytes: %w", io.ErrUnexpectedEOF)
	}

	return size, nil
}

// copyWithBuffer is very similar to  io.CopyBuffer https://golang.org/pkg/io/#CopyBuffer
// but instead of using Read to read from the src, we use ReadAtLeast to make sure we have
// a full buffer before we do a write operation to dst to reduce overheads associated
// with the write operations of small buffers.
func copyWithBuffer(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := pool.Get()
	defer pool.Put(buf)

	for {
		nr, er := io.ReadFull(src, buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			// If an EOF happens after reading fewer than the requested bytes,
			// ReadAtLeast returns ErrUnexpectedEOF.
			if er != io.EOF && er != io.ErrUnexpectedEOF {
				err = er
			}
			break
		}
	}
	return
}
