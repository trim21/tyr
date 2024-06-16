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
)

func (c *Client) Start() error {
	if err := c.startListen(); err != nil {
		return err
	}
	if log.Trace().Enabled() {
		go func() {
			for {
				time.Sleep(5 * time.Second)
				fmt.Println("goroutine count", runtime.NumGoroutine())
			}
		}()
	}

	log.Info().Msgf("using peer id prefix '%s'", global.PeerIDPrefix)
	for {
		time.Sleep(time.Second)
		c.m.RLock()
		for _, d := range c.downloads {
			if d.CouldAnnounce() {
				_ = global.Pool.Submit(func() {
					d.AsyncAnnounce(c.http)
				})
			}
		}
		c.m.RUnlock()
	}
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

			if c.mseDisabled {
				c.connChan <- conn
				continue
			}

			// handle mse
			c.m.RLock()
			keys := maps.Keys(c.downloadMap)
			c.m.RUnlock()

			rwc, err := mse.NewAccept(conn, keys, mse.PreferCrypto)
			if err != nil {
				continue
			}

			c.connChan <- rwc
		}
	}()
	return nil
}

func (c *Client) Shutdown() {
	c.cancl()
}
