package proto

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
