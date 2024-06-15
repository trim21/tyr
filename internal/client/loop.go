package client

import (
	"time"

	"github.com/rs/zerolog/log"

	"tyr/global"
)

func (c *Client) Start() {
	log.Info().Msgf("using peer id prefix '%s'", global.PeerIDPrefix)
	for {
		time.Sleep(time.Second)
		c.m.RLock()
		for _, d := range c.downloads {
			if d.CouldAnnounce() {
				d.AsyncAnnounce()
			}
		}
		c.m.RLock()
	}
}
