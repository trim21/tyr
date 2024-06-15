package peer

import "github.com/dchest/uniuri"

type ID [20]byte

var peerIDChars = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")

func NewID() ID {
	var peerID = make([]byte, 20)

	copy(peerID, peerIDPrefix)

	copy(peerID[8:], uniuri.NewLenCharsBytes(12, peerIDChars))

	return ID(peerID)
}
