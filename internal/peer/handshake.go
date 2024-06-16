package peer

import (
	"tyr/internal/proto"
)

func (p *Peer) Handshake() (proto.Handshake, error) {
	peerID := NewID()
	if err := proto.SendHandshake(p.Conn, p.InfoHash, peerID); err != nil {
		return proto.Handshake{}, err
	}

	return proto.ReadHandshake(p.Conn)
}
