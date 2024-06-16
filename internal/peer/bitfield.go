package peer

import (
	"github.com/negrel/assert"
	"github.com/rs/zerolog/log"

	"tyr/internal/proto"
	"tyr/internal/util"
)

func (p *Peer) decodeBitfield(l uint32) (Event, error) {
	if l != p.bitmapLen {
		return Event{}, ErrPeerSendInvalidData
	}

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
