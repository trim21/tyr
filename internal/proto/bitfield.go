package proto

import (
	"encoding/binary"
	"io"

	"tyr/internal/pkg/bm"
)

func SendBitfield(conn io.Writer, bm *bm.Bitmap, size uint32) error {
	err := binary.Write(conn, binary.BigEndian, 1+size)
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte{byte(Bitfield)})

	if err != nil {
		return err
	}

	b := bm.Bitfield(size)

	_, err = conn.Write(b[:size])

	return err
}
