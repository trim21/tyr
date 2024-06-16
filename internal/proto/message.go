package proto

//go:generate stringer -type=Message
type Message byte

// KeepAlive is a fake message to send keep alive event in application
// never hardcode this
const (
	Choke         Message = 0
	Unchoke       Message = 1
	Interested    Message = 2
	NotInterested Message = 3
	Have          Message = 4
	Bitfield      Message = 5
	Request       Message = 6
	Piece         Message = 7
	Cancel        Message = 8

	// BEP 5, for DHT

	Port Message = 9

	// BEP 6 - Fast extension

	Suggest     Message = 0x0d // 13
	HaveAll     Message = 0x0e // 14
	HaveNone    Message = 0x0f // 15
	Reject      Message = 0x10 // 16
	AllowedFast Message = 0x11 // 17

	// BEP 10

	Extended Message = 20

	// BEP 52

	HashRequest Message = 21
	Hashes      Message = 22
	HashReject  Message = 23

	BitCometExtension Message = 0xff
)
