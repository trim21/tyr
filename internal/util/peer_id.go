package util

import "github.com/dchest/uniuri"

var peerIDChars = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")

func NewPeerID() [20]byte {
	var peerID = make([]byte, 20)

	copy(peerID, peerIDPrefix)

	copy(peerID[8:], uniuri.NewLenCharsBytes(12, peerIDChars))

	return [20]byte(peerID)
}
