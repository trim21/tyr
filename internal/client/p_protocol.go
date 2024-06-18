package client

import (
	"encoding/binary"

	"github.com/negrel/assert"

	"tyr/internal/proto"
)

func (p *Peer) Handshake() (proto.Handshake, error) {
	if err := proto.SendHandshake(p.Conn, p.InfoHash, NewPeerID()); err != nil {
		return proto.Handshake{}, err
	}

	return proto.ReadHandshake(p.Conn)
}

func (p *Peer) decodeHave(l uint32) (Event, error) {
	assert.Equal(uint32(5), l)

	var b = make([]byte, l-1)
	n, err := p.Conn.Read(b)
	if err != nil {
		return Event{}, err
	}

	assert.Equal(4, n)

	return Event{Event: proto.Have, Index: binary.BigEndian.Uint32(b)}, err
}
