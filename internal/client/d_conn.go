package client

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"time"

	"tyr/internal/mse"
	"tyr/internal/pkg/global"
	"tyr/internal/proto"
)

// AddConn add an incoming connection from client listener
func (d *Download) AddConn(addr netip.AddrPort, conn net.Conn, h proto.Handshake) {
	//d.connMutex.Lock()
	//defer d.connMutex.Unlock()
	d.connectionHistory.Store(addr, connHistory{})
	d.conn.Store(addr, NewIncomingPeer(conn, d, addr, h))
}

func (d *Download) connectToPeers() {
	d.peersMutex.RLock()
	peers := slices.Clone(d.peers)
	d.peersMutex.RUnlock()

	for _, addr := range peers {
		if item := d.c.ch.Get(addr); item != nil {
			ch := item.Value()
			if ch.timeout {
				continue
			}
			if ch.err != nil {
				continue
			}
		}

		if _, ok := d.conn.Load(addr); ok {
			continue
		}

		if !d.c.sem.TryAcquire(1) {
			break
		}

		d.c.connectionCount.Add(1)

		global.Pool.Submit(func() {
			ch := connHistory{lastTry: time.Now()}
			defer func(h connHistory) {
				d.c.ch.Set(addr, h, time.Hour)
			}(ch)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			conn, err := global.Dialer.DialContext(ctx, "tcp", addr.String())
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					ch.timeout = true
				} else {
					ch.err = err
				}
				d.c.sem.Release(1)
				d.c.connectionCount.Sub(1)
				return
			}

			if d.c.mseDisabled {
				d.conn.Store(addr, NewOutgoingPeer(conn, d, addr))
				return
			}

			rwc, err := mse.NewConnection(d.info.Hash.Bytes(), conn)
			if err != nil {
				ch.err = err
				d.c.sem.Release(1)
				d.c.connectionCount.Sub(1)
				return
			}

			d.conn.Store(addr, NewOutgoingPeer(rwc, d, addr))
		})
	}
}
