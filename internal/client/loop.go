package client

import (
	"fmt"
	"runtime"
	"time"

	"github.com/rs/zerolog/log"

	"tyr/internal/pkg/global"
)

func (c *Client) Start() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			fmt.Println("goroutine count", runtime.NumGoroutine())
		}
	}()

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
