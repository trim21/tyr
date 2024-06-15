package proto

import (
	"bufio"
	"encoding/binary"
	"io"

	"tyr/internal/proto/size"
)

func SendPiece(conn io.Writer, pieceIndex, begin uint32, data []byte) error {
	w := bufio.NewWriter(conn)

	var buf = make([]byte, 0, size.Byte+size.Uint32*3)

	buf = binary.BigEndian.AppendUint32(buf, uint32(len(data)+size.Byte+size.Uint32*2))
	buf = append(buf, byte(Piece))
	buf = binary.BigEndian.AppendUint32(buf, pieceIndex)
	buf = binary.BigEndian.AppendUint32(buf, begin)

	w.Write(buf)
	w.Write(data)

	return w.Flush()
}
