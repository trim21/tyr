package proto

import (
	"encoding/binary"
	"io"

	"tyr/internal/pkg/bm"
)

func SendBitfield(conn io.Writer, bm *bm.Bitmap) error {
	b := bm.Bitfield()

	err := binary.Write(conn, binary.BigEndian, uint32(1+len(b)))
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte{byte(Bitfield)})

	if err != nil {
		return err
	}

	_, err = conn.Write(b)

	return err
}
