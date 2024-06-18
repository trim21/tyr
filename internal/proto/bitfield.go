package proto

import (
	"encoding/binary"
	"io"

	"tyr/internal/pkg/bm"
	"tyr/internal/util"
)

func NewBitfield(conn io.Writer, bm *bm.Bitmap, piecesLen int) error {
	chunked := util.BitmapToChunked(bm, piecesLen)

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
