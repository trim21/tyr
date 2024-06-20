package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/negrel/assert"
	"github.com/trim21/errgo"

	"tyr/internal/pkg/bm"
	"tyr/internal/proto"
)

func (p *Peer) keepAlive() {
	p.log.Trace().Msg("keep alive")
	timer := time.NewTimer(time.Minute * 2)
	defer p.cancel()
	defer timer.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-timer.C:
			t := p.lastSend.Load()
			// lastSend not set, doing handshake
			if t == nil {
				continue
			}
			if time.Now().Add(-time.Minute * 2).After(*t) {
				err := p.sendEvent(Event{keepAlive: true})
				if err != nil {
					return
				}
			}
		}
	}
}

func (p *Peer) DecodeEvents() (Event, error) {
	p.Conn.SetReadDeadline(time.Now().Add(time.Minute * 3))

	var b [4]byte
	n, err := io.ReadFull(p.Conn, b[:])
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 4)

	size := binary.BigEndian.Uint32(b[:])

	// keep alive
	if size == 0 {
		// keep alive
		return Event{keepAlive: true}, nil
	}

	p.log.Trace().Msgf("try to decode message with length %d", size)
	n, err = p.Conn.Read(b[:1])
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 1)

	evt := proto.Message(b[0])
	var event Event
	p.log.Trace().Msgf("try to decode message event '%s'", evt)
	switch evt {
	case proto.Interested, proto.NotInterested, proto.Choke,
		proto.Unchoke, proto.HaveAll, proto.HaveNone:
		return Event{Event: evt}, nil
	case proto.Bitfield:
		return p.decodeBitfield(size)
	case proto.Request:
		return p.decodeRequest()
	case proto.Cancel:
		return p.decodeCancel()
	case proto.Piece:
		return p.decodePiece(size)
	case proto.Port:
		err = binary.Read(p.Conn, binary.BigEndian, event.Port)
		return event, err
	case proto.Have, proto.Suggest, proto.AllowedFast:
		err = binary.Read(p.Conn, binary.BigEndian, event.Index)
		return event, err
	case proto.Reject:
		return p.decodeReject()
	case proto.Extended:
	case proto.BitCometExtension:
	}

	// unknown events
	_, err = io.CopyN(io.Discard, p.Conn, int64(size-1))
	return Event{Event: evt}, err
}

func (p *Peer) decodeBitfield(l uint32) (Event, error) {
	l = l - 1

	if l != p.bitfieldSize {
		return Event{}, errgo.Wrap(ErrPeerSendInvalidData,
			fmt.Sprintf("expecting bitfield length %d, receive %d", p.bitfieldSize, l))
	}

	var b = make([]byte, l+8)
	n, err := io.ReadFull(p.Conn, b[:l])
	if err != nil {
		return Event{}, err
	}
	assert.Equal(n, int(l))

	bmLen := l/8 + 8

	var bb = make([]uint64, bmLen)
	for i := uint32(0); i < bmLen; i += 1 {
		bb[i] = binary.BigEndian.Uint64(b[i : i+8])
	}

	bitmap := roaring.FromDense(bb, false)

	bitmap.RemoveRange(uint64(p.d.numPieces), uint64(p.d.numPieces+64*8))

	return Event{Event: proto.Bitfield, Bitmap: bm.FromBitmap(bitmap)}, nil
}

func (p *Peer) decodeCancel() (Event, error) {
	payload, err := proto.ReadCancelPayload(p.Conn)
	if err != nil {
		return Event{}, err
	}

	return Event{Event: proto.Cancel, Req: payload}, err
}

func (p *Peer) decodeRequest() (Event, error) {
	payload, err := proto.ReadRequestPayload(p.Conn)
	if err != nil {
		return Event{}, err
	}

	return Event{Event: proto.Request, Req: payload}, err
}

func (p *Peer) decodeReject() (Event, error) {
	payload, err := proto.ReadRequestPayload(p.Conn)
	if err != nil {
		return Event{}, err
	}

	return Event{Event: proto.Reject, Req: payload}, err
}
