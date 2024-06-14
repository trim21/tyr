package proto

import (
	"encoding/binary"
	"io"

	"github.com/kelindar/bitmap"
	"github.com/negrel/assert"

	"ve/internal/util"
)

func NewBitfield(conn io.Writer, bm bitmap.Bitmap, piecesLen int) error {
	chunked := util.BitmapToChunked(bm, piecesLen)

	assert.Len(chunked, 300)

	err := binary.Write(conn, binary.BigEndian, uint32(1+len(chunked)))
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte{byte(Bitfield)})
	if err != nil {
		return err
	}

	_, err = conn.Write(chunked)

	return err
}
