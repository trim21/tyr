package proto

import (
	"encoding/binary"

	"github.com/negrel/assert"
)

//go:generate stringer -type=Message
type Message uint8

const Choke Message = 0
const Unchoke Message = 1
const Interested Message = 2
const NotInterested Message = 3
const Have Message = 4
const Bitfield Message = 5
const Request Message = 6
const Piece Message = 7
const Cancel Message = 8
const Port Message = 9
const BitCometExtension Message = 0xff

func NewHave(pieceIndex uint32) []byte {
	var b = make([]byte, 0, 9)
	b = binary.BigEndian.AppendUint32(b, 5)
	b = append(b, byte(Have))
	b = binary.BigEndian.AppendUint32(b, pieceIndex)
	assert.Len(b, 4+1+4)
	return b
}

func NewRequest(pieceIndex uint32, begin uint32, length uint32) []byte {
	var b = make([]byte, 0, 4+4+4+1+4)
	b = binary.BigEndian.AppendUint32(b, 4+4+4+1)

	b = append(b, byte(Request))

	b = binary.BigEndian.AppendUint32(b, pieceIndex)
	b = binary.BigEndian.AppendUint32(b, begin)
	b = binary.BigEndian.AppendUint32(b, length)

	assert.Len(b, 4+4+4+1+4)

	return b
}

func NewCancel(pieceIndex uint32, begin uint32, length uint32) []byte {
	var b = make([]byte, 0, 4+1+4+4+4)
	b = binary.BigEndian.AppendUint32(b, 13)

	b = append(b, byte(Cancel))

	b = binary.BigEndian.AppendUint32(b, pieceIndex)
	b = binary.BigEndian.AppendUint32(b, begin)
	b = binary.BigEndian.AppendUint32(b, length)

	assert.Len(b, 4+1+4+4+4)

	return b
}

func NewPort(port uint16) []byte {
	var b = make([]byte, 0, 4+1+2)
	b = binary.BigEndian.AppendUint32(b, 3)

	b = append(b, byte(Port))

	b = binary.BigEndian.AppendUint16(b, port)

	assert.Len(b, 7)

	return b
}
