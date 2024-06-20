package proto

import (
	"encoding/binary"
	"io"

	"tyr/internal/pkg/bufpool"
	"tyr/internal/proto/size"
)

func SendPiece(conn io.Writer, r ChunkResponse) error {
	var buf = bufpool.Get()
	defer bufpool.Put(buf)

	buf.B = binary.BigEndian.AppendUint32(buf.B, uint32(len(r.Data)+size.Byte+size.Uint32*2))
	buf.B = append(buf.B, byte(Piece))
	buf.B = binary.BigEndian.AppendUint32(buf.B, r.PieceIndex)
	buf.B = binary.BigEndian.AppendUint32(buf.B, r.Begin)

	_, err := conn.Write(buf.B)
	if err != nil {
		return err
	}

	_, err = conn.Write(r.Data)
	return err
}

func ReadPiecePayload(conn io.Reader, size uint32) (ChunkResponse, error) {
	var payload = ChunkResponse{
		Begin:      0,
		PieceIndex: 0,
		Data:       make([]byte, size-sizeUint32*2),
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
