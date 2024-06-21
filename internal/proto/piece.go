package proto

import (
	"encoding/binary"
	"io"
)

type ChunkResponse struct {
	// len(Data) should match request
	Data       []byte
	Begin      uint32
	PieceIndex uint32
}

func SendPiece(conn io.Writer, r ChunkResponse) error {
	var b [sizeUint32 + sizeByte + sizeUint32*2]byte

	binary.BigEndian.PutUint32(b[:], uint32(len(r.Data)+sizeByte+sizeUint32*2))
	b[4] = byte(Piece)
	binary.BigEndian.PutUint32(b[sizeUint32+sizeByte:], r.PieceIndex)
	binary.BigEndian.PutUint32(b[sizeUint32+sizeByte+sizeUint32:], r.Begin)

	_, err := conn.Write(b[:])
	if err != nil {
		return err
	}

	_, err = conn.Write(r.Data)
	return err
}

func ReadPiecePayload(conn io.Reader, size uint32) (ChunkResponse, error) {
	var payload = ChunkResponse{
		Data: make([]byte, size-sizeUint32*2),
	}

	err := binary.Read(conn, binary.BigEndian, &payload.PieceIndex)
	if err != nil {
		return ChunkResponse{}, err
	}

	err = binary.Read(conn, binary.BigEndian, &payload.Begin)
	if err != nil {
		return ChunkResponse{}, err
	}

	_, err = io.ReadFull(conn, payload.Data)

	return payload, err
}
