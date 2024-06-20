package client

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchest/uniuri"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"github.com/samber/lo"

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

func NewOutgoingPeer(conn net.Conn, d *Download, addr netip.AddrPort) *Peer {
	return newPeer(conn, d, addr, emptyPeerID, false)
}

func NewIncomingPeer(conn net.Conn, d *Download, addr netip.AddrPort, peerID PeerID) *Peer {
	return newPeer(conn, d, addr, peerID, true)
}

func newPeer(
	conn net.Conn,
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
		ctx:          ctx,
		log:          l.Logger(),
		Conn:         conn,
		d:            d,
		cancel:       cancel,
		bitfieldSize: util.BitmapLen(d.numPieces),
		Bitmap:       bm.New(),
		Address:      addr,
		reqChan:      make(chan req.Request, 1),
		//ResChan:   make(chan req.Response, 1),
		requests: xsync.MapOf[req.Request, empty.Empty]{},
	}

	go p.start(skipHandshake)
	return p
}

var ErrPeerSendInvalidData = errors.New("peer send invalid data")

type Peer struct {
	log          zerolog.Logger
	ctx          context.Context
	Conn         net.Conn
	d            *Download
	lastSend     atomic.Pointer[time.Time]
	reqChan      chan req.Request
	cancel       context.CancelFunc
	Bitmap       *bm.Bitmap
	requests     xsync.MapOf[req.Request, empty.Empty]
	Address      netip.AddrPort
	m            sync.Mutex
	wm           sync.Mutex
	dead         atomic.Bool
	bitfieldSize uint32
	Choked       atomic.Bool
	Interested   atomic.Bool
}

type Event struct {
	Bitmap    *bm.Bitmap
	Res       req.Response
	Req       req.Request
	Index     uint32
	Event     proto.Message
	keepAlive bool
	Port      uint16
}

func (p *Peer) start(skipHandshake bool) {
	p.log.Trace().Msg("start")
	defer p.log.Trace().Msg("close")
	defer close(p.reqChan)
	defer p.Conn.Close()
	defer p.d.conn.Delete(p.Address)
	defer p.d.c.sem.Release(1)
	defer p.d.c.connectionCount.Sub(1)
	defer p.cancel()
	defer p.dead.Store(true)

	if err := proto.SendHandshake(p.Conn, p.d.hash, NewPeerID()); err != nil {
		p.log.Trace().Err(err).Msg("failed to send handshake to peer")
		return
	}

	if !skipHandshake {
		h, err := proto.ReadHandshake(p.Conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				p.log.Trace().Err(err).Msg("failed to read handshake from peer")
			}
			return
		}
		if h.InfoHash != p.d.hash {
			p.log.Trace().Msgf("peer info hash mismatch %x", h.InfoHash)
			return
		}
		p.log = p.log.With().Str("peer_id", url.QueryEscape(string(h.PeerID[:]))).Logger()
		p.log.Trace().Msg("connect to peer")
	}

	if err := p.sendEvent(Event{Event: proto.Bitfield, Bitmap: p.d.bm.Clone()}); err != nil {
		return
	}

	go p.keepAlive()

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case q := <-p.reqChan:
				p.requests.Store(q, empty.Empty{})
				err := p.sendEvent(Event{
					Event: proto.Request,
					Req:   q,
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
			if !errors.Is(err, io.EOF) {
				p.log.Trace().Err(err).Msg("failed to decode event")
			}
			return
		}

		p.log.Trace().Msgf("receive %s event", event.Event)

		switch event.Event {
		case proto.Bitfield:
			p.Bitmap.OR(event.Bitmap)
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
				p.log.Trace().Msg("failed to validate response")
				// send response without requests
				return
			}
			p.d.ResChan <- event.Res
		case proto.Request:
			p.reqChan <- event.Req

		// TODO
		case proto.Cancel:
		case proto.Port:
		case proto.Suggest:
		case proto.HaveAll:
		case proto.HaveNone:
		case proto.Reject:
		case proto.AllowedFast:
		// currently unsupported
		case proto.Extended:
		// currently ignored
		case proto.BitCometExtension:
		}
	}
}

func (p *Peer) sendEvent(e Event) error {
	p.wm.Lock()
	defer p.wm.Unlock()

	p.Conn.SetWriteDeadline(time.Now().Add(time.Minute * 3))

	p.lastSend.Store(lo.ToPtr(time.Now()))

	if e.keepAlive {
		return proto.SendKeepAlive(p.Conn)
	}

	switch e.Event {
	case proto.Choke:
		return proto.SendChoke(p.Conn)
	case proto.Unchoke:
		return proto.SendUnchoke(p.Conn)
	case proto.Interested:
		return proto.SendInterested(p.Conn)
	case proto.NotInterested:
		return proto.SendNotInterested(p.Conn)
	case proto.Have:
		return proto.SendHave(p.Conn, e.Index)
	case proto.Bitfield:
		return proto.SendBitfield(p.Conn, e.Bitmap, p.bitfieldSize)
	case proto.Request:
		return proto.SendRequest(p.Conn, e.Req)
	case proto.Piece:
		return proto.SendPiece(p.Conn, e.Res)
	case proto.Cancel:
		return proto.SendCancel(p.Conn, e.Req)
	case proto.Port:
		return proto.SendPort(p.Conn, e.Port)
	case proto.Suggest:
		return proto.SendSuggest(p.Conn, e.Index)
	case proto.HaveAll, proto.HaveNone:
		return proto.SendNoPayload(p.Conn, e.Event)
	case proto.AllowedFast:
		return proto.SendIndexOnly(p.Conn, e.Event, e.Index)
	case proto.Reject:
		return proto.SendReject(p.Conn, e.Req)
	case proto.Extended, proto.BitCometExtension:
		panic("unexpected event")
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

func (p *Peer) decodePiece(size uint32) (Event, error) {
	payload, err := proto.ReadPiecePayload(p.Conn, size)
	if err != nil {
		return Event{}, err
	}

	return Event{Event: proto.Piece, Res: payload}, nil
}
