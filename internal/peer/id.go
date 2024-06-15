package peer

import (
	"github.com/dchest/uniuri"

	"tyr/global"
)

type ID [20]byte

var peerIDChars = []byte("0123456789abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")

func NewID() ID {
	var peerID = make([]byte, 20)

	copy(peerID, global.PeerIDPrefix)

	copy(peerID[8:], uniuri.NewLenCharsBytes(12, peerIDChars))

	return ID(peerID)
}
