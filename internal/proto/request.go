package proto

import (
	"encoding/binary"
	"io"

	"github.com/negrel/assert"

	"tyr/internal/proto/size"
)

type ChunkRequest struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

type ChunkResponse struct {
	// len(Data) should match request
	Data       []byte
	Begin      uint32
	PieceIndex uint32
}

func SendRequest(conn io.Writer, request ChunkRequest) error {
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

func SendCancel(conn io.Writer, request ChunkRequest) error {
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

func SendReject(conn io.Writer, request ChunkRequest) error {
	var b = make([]byte, 0, 4+4+4+1+4)
	b = binary.BigEndian.AppendUint32(b, 4+4+4+1)

	b = append(b, byte(Reject))

	b = binary.BigEndian.AppendUint32(b, request.PieceIndex)
	b = binary.BigEndian.AppendUint32(b, request.Begin)
	b = binary.BigEndian.AppendUint32(b, request.Length)

	assert.Len(b, 4+4+4+1+4)

	_, err := conn.Write(b)
	return err
}

func ReadRequestPayload(conn io.Reader) (payload ChunkRequest, err error) {
	var b [12]byte

	_, err = io.ReadFull(conn, b[:])
	if err != nil {
		return
	}

	payload.PieceIndex = binary.BigEndian.Uint32(b[:4])
	payload.Begin = binary.BigEndian.Uint32(b[size.Uint32:])
	payload.Length = binary.BigEndian.Uint32(b[size.Uint32*2:])

	return
}

func ReadCancelPayload(conn io.Reader) (ChunkRequest, error) {
	return ReadRequestPayload(conn)
}
