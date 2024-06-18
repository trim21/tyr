package client

import (
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/trim21/errgo"
	"golang.org/x/exp/maps"

	"tyr/internal/mse"
	"tyr/internal/pkg/global"
	"tyr/internal/proto"
)

func (c *Client) Start() error {
	if err := c.startListen(); err != nil {
		return err
	}

	if log.Trace().Enabled() {
		go func() {
			for {
				time.Sleep(time.Second * 5)
				fmt.Println("goroutine count", runtime.NumGoroutine())
				fmt.Println("connection count", c.connectionCount.Load())
				c.m.RLock()
				fmt.Println("show downloads")
				for _, d := range c.downloads {
					fmt.Println(d.Display())
				}
				c.m.RUnlock()
			}
		}()
	}

	go func() {
		for {
			time.Sleep(time.Minute * 10)
			c.m.RLock()
			log.Info().Msg("save session")
			err := c.saveSession()
			c.m.RUnlock()
			if err != nil {
				fmt.Println(string(err.Stack))
			}
		}
	}()

	//go func() {
	//	log.Info().Msgf("using peer id prefix '%s'", global.PeerIDPrefix)
	//	for {
	//		time.Sleep(time.Second)
	//		c.m.RLock()
	//		for _, d := range c.downloads {
	//			d.m.RLock()
	//			if !(d.state == Uploading || d.state == Downloading) {
	//				d.m.RUnlock()
	//				continue
	//			}
	//			d.m.RUnlock()
	//
	//			if d.CouldAnnounce() {
	//				global.Pool.Submit(func() {
	//					d.AsyncAnnounce(c.http)
	//				})
	//			}
	//		}
	//		c.m.RUnlock()
	//	}
	//}()

	return nil
}

func (c *Client) startListen() error {
	var lc net.ListenConfig
	l, err := lc.Listen(c.ctx, "tcp", fmt.Sprintf(":%d", c.Config.App.P2PPort))
	if err != nil {
		return errgo.Wrap(err, "failed to listen on p2p port")
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}

			if !c.sem.TryAcquire(1) {
				_ = conn.Close()
				continue
			}

			c.connectionCount.Add(1)
			if c.mseDisabled {
				c.connChan <- incomingConn{
					addr: conn.RemoteAddr().String(),
					conn: conn,
				}
				continue
			}

			// handle mse
			global.Pool.Submit(func() {
				c.m.RLock()
				keys := maps.Keys(c.downloadMap)
				c.m.RUnlock()

				rwc, err := mse.NewAccept(conn, keys, c.mseSelector)
				if err != nil {
					c.connectionCount.Sub(1)
					c.sem.Release(1)
					c.connectionCount.Sub(1)
					return
				}

				c.connChan <- incomingConn{
					addr: conn.RemoteAddr().String(),
					conn: rwc,
				}
			})
		}
	}()
	return nil
}

func (c *Client) handleConn() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case conn := <-c.connChan:
			global.Pool.Submit(func() {
				h, err := proto.ReadHandshake(conn.conn)
				if err != nil {
					_ = conn.conn.Close()
					c.sem.Release(1)
					c.connectionCount.Sub(1)
				}

				log.Debug().Stringer("info_hash", h.InfoHash).Msg("incoming connection")

				c.m.RLock()
				defer c.m.RUnlock()

				d, ok := c.downloadMap[h.InfoHash]
				if !ok {
					_ = conn.conn.Close()
					return
				}

				d.AddConn(conn.addr, conn.conn)
			})
			continue
		}
	}
}
