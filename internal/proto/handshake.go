package proto

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/negrel/assert"

	"tyr/internal/meta"
)

var handshakePstrV1 = []byte("\x13BitTorrent protocol")

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
	_, err := conn.Write(handshakePstrV1)
	if err != nil {
		return err
	}

	_, err = conn.Write(HandshakeReserved)
	if err != nil {
		return err
	}

	_, err = conn.Write(infoHash[:])
	if err != nil {
		return err
	}

	_, err = conn.Write(peerID[:])
	return err
}

type Handshake struct {
	InfoHash      meta.Hash
	PeerID        [20]byte
	FastExtension bool
}

func (h Handshake) GoString() string {
	return fmt.Sprintf("Handshake{InfoHash='%x', PeerID='%s'}", h.InfoHash, h.PeerID)
}

var ErrHandshakeMismatch = errors.New("handshake string mismatch")

var fastExtensionEnabled byte = 1 << 2

func ReadHandshake(conn io.Reader) (Handshake, error) {
	var b = make([]byte, 20)
	n, err := io.ReadFull(conn, b)
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(20, n)

	if !bytes.Equal(b, handshakePstrV1) {
		return Handshake{}, ErrHandshakeMismatch
	}

	n, err = io.ReadFull(conn, b[:8])
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(8, n)

	var h = Handshake{}

	if b[8]&fastExtensionEnabled != 0 {
		h.FastExtension = true
	}

	n, err = conn.Read(h.InfoHash[:])
	if err != nil {
		return Handshake{}, err
	}
	assert.Equal(20, n)

	n, err = conn.Read(h.PeerID[:])
	if err != nil {
		return Handshake{}, err
	}

	assert.Equal(20, n)

	return h, nil
}
