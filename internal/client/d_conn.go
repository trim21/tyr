package client

import (
	"io"
	"net"
	"time"

	"github.com/rs/zerolog/log"

	"tyr/internal/peer"
	"tyr/internal/pkg/global"
)

// AddConn add an incoming connection from client listener
func (d *Download) AddConn(addr string, conn io.ReadWriteCloser) {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	d.conn.Store(addr, peer.NewIncoming(conn, d.infoHash, d.numPieces, addr))
}

func (d *Download) cleanDeadPeers() {
	d.conn.Range(func(key string, value *peer.Peer) bool {
		if value.Dead() {
			d.conn.Delete(key)
			d.c.sem.Release(1)
			d.c.connectionCount.Add(1)
		}
		return true
	})
}

func (d *Download) connectToPeers() {
	d.cleanDeadPeers()

	d.peersMutex.RLock()
	if len(d.peers) > 0 {
		log.Trace().Msg("connectToPeers")
	}

	for _, p := range d.peers {
		a := p.String()
		log.Trace().Msgf("check peer %s", a)
		_, connected := d.conn.Load(a)

		if connected {
			log.Trace().Msgf("peer connected")
			continue
		}

		h, ok := d.connectionHistory.Load(a)
		if ok {
			if h.lastTry.After(time.Now().Add(-time.Minute)) {
				continue
			}
		}

		if !d.c.sem.TryAcquire(1) {
			break
		}
		d.c.connectionCount.Add(1)

		conn, err := global.Dialer.Dial("tcp", a)
		if err != nil {
			d.connectionHistory.Store(a, connHistory{lastTry: time.Now()})
			continue
		}
		d.connectionHistory.Store(a, connHistory{lastTry: time.Now(), connected: true})

		log.Trace().Msgf("connected to peer %s", a)
		d.conn.Store(a, d.peerFromConn(conn, a))
	}

	d.peersMutex.RUnlock()

	d.peersMutex.Lock()
	d.peers = d.peers[:0]
	d.peersMutex.Unlock()
}

func (d *Download) peerFromConn(conn net.Conn, a string) *peer.Peer {
	return peer.New(conn, d.infoHash, uint32(d.info.NumPieces()), a)
}
