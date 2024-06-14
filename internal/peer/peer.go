package peer

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/kelindar/bitmap"
	"github.com/rs/zerolog/log"

	"github.com/negrel/assert"

	"ve/internal/proto"
	"ve/internal/util"
)

func New(conn net.Conn, infoHash [20]byte, pieceNum uint32) Peer {
	return Peer{Conn: conn, InfoHash: infoHash, PieceNum: pieceNum, M: &sync.Mutex{}}
}

type Peer struct {
	Conn     net.Conn
	M        *sync.Mutex
	PieceNum uint32
	InfoHash [20]byte
	Bitmap   bitmap.Bitmap
}

func (p Peer) bitmapLen() int {
	if p.PieceNum%8 == 0 {
		return int(p.PieceNum / 8)
	}

	return int(8 * (p.PieceNum/8 + 1))
}

func (p Peer) Handshake() (proto.Handshake, error) {
	peerID := util.NewPeerID()
	fmt.Printf("current peer id %s\n", peerID)
	if err := proto.SendHandshake(p.Conn, p.InfoHash, peerID); err != nil {
		return proto.Handshake{}, err
	}

	return proto.ReadHandshake(p.Conn)
}

type Event struct {
	Event proto.Message

	Bitmap bitmap.Bitmap
}

func (p Peer) DecodeEvents() (Event, error) {
	var b = make([]byte, 4)
	n, err := p.Conn.Read(b)
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 4)

	l := binary.BigEndian.Uint32(b)
	fmt.Println("len", l)

	if l == 0 {
		// keep alive
		return Event{}, nil
	}

	log.Trace().Msgf("try to decode message with length %d", l)
	n, err = p.Conn.Read(b[:1])
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 1)

	evt := proto.Message(b[0])
	log.Trace().Msgf("try to decode message event '%s'", evt)
	switch evt {
	case proto.Bitfield:
		return p.decodeBitfield(l)
	}

	_, err = io.CopyN(io.Discard, p.Conn, int64(l-1))
	return Event{}, err
}

func (p Peer) decodeBitfield(l uint32) (Event, error) {
	// TODO: verify bitfield length with torrent data

	var b = make([]byte, l-1)
	n, err := p.Conn.Read(b)
	if err != nil {
		return Event{}, err
	}

	log.Trace().Msgf("receive bitfield payload length %d", l-1)

	assert.Equal(n, int(l-1))

	bm := util.BitmapFromChunked(b)

	return Event{Event: proto.Bitfield, Bitmap: bm}, err
}
