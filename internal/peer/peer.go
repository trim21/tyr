package peer

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/anacrolix/torrent"
	"github.com/kelindar/bitmap"
	"github.com/puzpuzpuz/xsync/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/negrel/assert"

	"tyr/internal/pkg/empty"
	"tyr/internal/proto"
	"tyr/internal/req"
	"tyr/internal/util"
)

func New(conn io.ReadWriteCloser, infoHash [20]byte, pieceNum uint32, addr string) *Peer {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Peer{
		ctx:       ctx,
		log:       log.With().Hex("info_hash", infoHash[:]).Str("addr", addr).Logger(),
		m:         sync.Mutex{},
		Conn:      conn,
		InfoHash:  infoHash,
		bitmapLen: util.BitmapLen(pieceNum),
		requests:  xsync.MapOf[req.Request, empty.Empty]{},
	}
	p.cancel = func() {
		p.dead.Store(true)
		cancel()
	}
	go p.start()
	return p
}

var ErrPeerSendInvalidData = errors.New("peer send invalid data")

type Peer struct {
	m sync.Mutex

	dead atomic.Bool

	log zerolog.Logger

	resChan chan<- req.Response
	reqChan chan req.Request

	requests xsync.MapOf[req.Request, empty.Empty]

	ctx    context.Context
	cancel context.CancelFunc

	Conn    io.ReadWriteCloser
	Address string

	// bitmap of connected peer
	Bitmap    bitmap.Bitmap
	bitmapLen uint32

	// torrent metainfo
	InfoHash torrent.InfoHash

	Choked     atomic.Bool
	Interested atomic.Bool
}

type Event struct {
	Bitmap bitmap.Bitmap
	Res    req.Response
	Req    req.Request
	Event  proto.Message
	Index  uint32

	keepAlive bool
}

func (p *Peer) DecodeEvents() (Event, error) {
	var b = make([]byte, 4)
	n, err := p.Conn.Read(b)
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 4)

	l := binary.BigEndian.Uint32(b)

	// keep alive
	if l == 0 {
		// keep alive
		return Event{}, nil
	}

	p.log.Trace().Msgf("try to decode message with length %d", l)
	n, err = p.Conn.Read(b[:1])
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 1)

	evt := proto.Message(b[0])
	p.log.Trace().Msgf("try to decode message event '%s'", evt)
	switch evt {
	case proto.Bitfield:
		return p.decodeBitfield(l)
	case proto.Have:
		return p.decodeHave(l)
	case proto.Interested, proto.NotInterested, proto.Choke, proto.Unchoke:
		return Event{Event: evt}, nil
	}

	// unknown events
	_, err = io.CopyN(io.Discard, p.Conn, int64(l-1))
	return Event{Event: evt}, err
}

func (p *Peer) start() {
	defer p.cancel()
	h, err := p.Handshake()
	if err != nil {
	}

	if h.InfoHash != p.InfoHash {
		p.log.Trace().Msgf("peer info hash mismatch %x", h.InfoHash)
		return
	}

	p.log.Trace().Msgf("connect to peer %s", url.QueryEscape(string(h.PeerID[:])))

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case r := <-p.reqChan:
				p.requests.Store(r, empty.Empty{})
				err := p.sendEvent(Event{
					Event: proto.Request,
					Req:   r,
				})
				// TODO: should handle error here
				if err != nil {
					return
				}
			}
		}
	}()

	for {
		if p.ctx.Err() != nil {
			return
		}
		event, err := p.DecodeEvents()
		if err != nil {
			if errors.Is(err, ErrPeerSendInvalidData) {
				_ = p.Conn.Close()
				return
			}
			_ = p.Conn.Close()
			return
		}

		switch event.Event {
		case proto.Bitfield:
			p.Bitmap.Xor(event.Bitmap)
		case proto.Have:
			p.Bitmap.Set(event.Index)
		case proto.Interested:
			p.Interested.Store(true)
		case proto.NotInterested:
			p.Interested.Store(false)
		case proto.Choke:
			p.Choked.Store(true)
		case proto.Unchoke:
			p.Choked.Store(false)
		case proto.Piece:
			if !p.validateRes(event.Res) {
				// send response without requests
				_ = p.Conn.Close()
				return
			}
			p.resChan <- event.Res
		case proto.Request:
			p.reqChan <- event.Req
		}

		p.log.Trace().Msgf("receive %s event", event.Event)
	}
}

func (p *Peer) sendEvent(event Event) error {
	p.m.Lock()
	defer p.m.Unlock()

	if event.keepAlive {
		return proto.SendKeepAlive(p.Conn)
	}

	switch event.Event {
	case proto.Request:
		return proto.SendRequest(p.Conn, event.Req)
	}

	return nil
}

func (p *Peer) validateRes(res req.Response) bool {
	r := req.Request{
		PieceIndex: res.PieceIndex,
		Begin:      res.Begin,
		Length:     uint32(len(res.Data)),
	}

	if _, ok := p.requests.Load(r); ok {
		p.requests.Delete(r)
		return true
	}
	return false
}

func (p *Peer) Dead() bool {
	return p.dead.Load()
}
