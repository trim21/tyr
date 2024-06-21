package proto

import (
	"encoding/binary"
	"io"

	"github.com/negrel/assert"

	"tyr/internal/pkg/bm"
)

func SendBitfield(conn io.Writer, bm *bm.Bitmap) error {
	b := bm.Bitfield()

	assert.Equal(2916, len(b))

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
