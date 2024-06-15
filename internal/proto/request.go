package proto

import (
	"encoding/binary"
	"io"

	"github.com/negrel/assert"

	"ve/internal/req"
)

func SendRequest(conn io.Writer, request req.Request) error {
	var b = make([]byte, 0, 4+4+4+1+4)
	b = binary.BigEndian.AppendUint32(b, 4+4+4+1)

	b = append(b, byte(Request))

	b = binary.BigEndian.AppendUint32(b, request.PieceIndex)
	b = binary.BigEndian.AppendUint32(b, request.Begin)
	b = binary.BigEndian.AppendUint32(b, request.Length)

	assert.Len(b, 4+4+4+1+4)

	_, err := conn.Write(b)
	return err
}

func SendCancel(conn io.Writer, request req.Request) error {
	var b = make([]byte, 0, 4+1+4+4+4)
	b = binary.BigEndian.AppendUint32(b, 13)

	b = append(b, byte(Cancel))

	b = binary.BigEndian.AppendUint32(b, request.PieceIndex)
	b = binary.BigEndian.AppendUint32(b, request.Begin)
	b = binary.BigEndian.AppendUint32(b, request.Length)

	assert.Len(b, 4+1+4+4+4)

	_, err := conn.Write(b)
	return err
}
