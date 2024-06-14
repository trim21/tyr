package proto

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/negrel/assert"
)

const HandshakePstrV1 = "BitTorrent protocol"
const HandshakePstrLen = byte(len(HandshakePstrV1))

var HandshakeReserved = []byte{0, 0, 0, 0, 0, 0, 0, 0}

// SendHandshake = <pStrlen><pStr><reserved><info_hash><peer_id>
// - pStrlen = length of pStr (1 byte)
// - pStr = string identifier of the protocol: "BitTorrent protocol" (19 bytes)
// - reserved = 8 reserved bytes indicating extensions to the protocol (8 bytes)
// - info_hash = hash of the value of the 'info' key of the torrent file (20 bytes)
// - peer_id = unique identifier of the Peer (20 bytes)
//
// Total length = payload length = 49 + len(pstr) = 68 bytes (for BitTorrent v1)
func SendHandshake(conn io.Writer, infoHash, peerID [20]byte) error {
	c := bufio.NewWriter(conn)

	c.WriteByte(HandshakePstrLen)

	c.WriteString(HandshakePstrV1)

	// reserved
	c.Write(HandshakeReserved)

	c.Write(infoHash[:])
	c.Write(peerID[:])

	return c.Flush()
}

type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

func (h Handshake) GoString() string {
	return fmt.Sprintf("Handshake{InfoHash='%x', PeerID='%s'}", h.InfoHash, h.PeerID)
}

var ErrHandshakeMismatch = errors.New("handshake string mismatch")

func ReadHandshake(conn io.Reader) (Handshake, error) {
	var b = make([]byte, 1)
	n, err := conn.Read(b)
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(n, 1)

	l := b[0]
	fmt.Println(l)

	b = make([]byte, l)
	n, err = conn.Read(b)
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(n, int(HandshakePstrLen))

	if string(b) != HandshakePstrV1 {
		return Handshake{}, ErrHandshakeMismatch
	}

	n, err = conn.Read(b[:8])
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(n, 8)

	var h = Handshake{}

	n, err = conn.Read(h.InfoHash[:])
	if err != nil {
		return Handshake{}, err
	}
	assert.Equal(n, 20)

	n, err = conn.Read(h.PeerID[:])
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(n, 20)

	return h, nil
}
