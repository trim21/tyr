package client

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/anacrolix/torrent"
	"github.com/dchest/uniuri"
	"github.com/negrel/assert"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"tyr/internal/pkg/bm"
	"tyr/internal/pkg/empty"
	"tyr/internal/pkg/global"
	"tyr/internal/pkg/unsafe"
	"tyr/internal/proto"
	"tyr/internal/req"
	"tyr/internal/util"
)

type PeerID [20]byte

func (i PeerID) AsString() string {
	return unsafe.Str(i[:])
}

var emptyPeerID PeerID

func (i PeerID) Zero() bool {
	return i == emptyPeerID
}

var peerIDChars = []byte("0123456789abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")

func NewPeerID() (peerID PeerID) {
	copy(peerID[:], global.PeerIDPrefix)
	copy(peerID[8:], uniuri.NewLenCharsBytes(12, peerIDChars))
	return
}

func NewOutgoingPeer(conn io.ReadWriteCloser, d *Download, addr netip.AddrPort) *Peer {
	return newPeer(conn, d, addr, emptyPeerID, false)
}

func NewIncomingPeer(conn io.ReadWriteCloser, d *Download, addr netip.AddrPort, peerID PeerID) *Peer {
	return newPeer(conn, d, addr, peerID, true)
}

func newPeer(
	conn io.ReadWriteCloser,
	d *Download,
	addr netip.AddrPort,
	peerID PeerID,
	skipHandshake bool,
) *Peer {
	ctx, cancel := context.WithCancel(context.Background())
	l := d.log.With().Stringer("addr", addr)
	if !peerID.Zero() {
		l = l.Str("peer_id", url.QueryEscape(peerID.AsString()))
	}

	p := &Peer{
		ctx:       ctx,
		log:       l.Logger(),
		Conn:      conn,
		InfoHash:  d.infoHash,
		bitmapLen: util.BitmapLen(d.numPieces),
		requests:  xsync.MapOf[req.Request, empty.Empty]{},
	}
	p.cancel = func() {
		p.dead.Store(true)
		d.conn.Delete(addr)
		d.c.sem.Release(1)
		d.c.connectionCount.Sub(1)
		cancel()
	}
	go p.start(skipHandshake)
	return p
}

var ErrPeerSendInvalidData = errors.New("peer send invalid data")

type Peer struct {
	log        zerolog.Logger
	ctx        context.Context
	Conn       io.ReadWriteCloser
	resChan    chan<- req.Response
	reqChan    chan req.Request
	cancel     context.CancelFunc
	requests   xsync.MapOf[req.Request, empty.Empty]
	Address    string
	Bitmap     *bm.Bitmap
	m          sync.Mutex
	dead       atomic.Bool
	bitmapLen  uint32
	Choked     atomic.Bool
	Interested atomic.Bool
	InfoHash   torrent.InfoHash
}

type Event struct {
	Bitmap    *bm.Bitmap
	Res       req.Response
	Req       req.Request
	Index     uint32
	Event     proto.Message
	keepAlive bool
}

func (p *Peer) DecodeEvents() (Event, error) {
	var b [4]byte
	n, err := p.Conn.Read(b[:])
	if err != nil {
		return Event{}, err
	}

	assert.Equal(n, 4)

	l := binary.BigEndian.Uint32(b[:])

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

func (p *Peer) start(skipHandshake bool) {
	defer p.cancel()
	if skipHandshake {
		if proto.SendHandshake(p.Conn, p.InfoHash, NewPeerID()) != nil {
			return
		}
	} else {
		h, err := p.Handshake()
		if err != nil {
			return
		}
		if h.InfoHash != p.InfoHash {
			p.log.Trace().Msgf("peer info hash mismatch %x", h.InfoHash)
			return
		}
		p.log = p.log.With().Str("peer_id", url.QueryEscape(string(h.PeerID[:]))).Logger()
		p.log.Trace().Msg("connect to peer")
	}

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
			p.Bitmap.XOR(event.Bitmap)
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
