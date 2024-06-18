package global

import (
	"fmt"
	"net"
	"time"

	"github.com/panjf2000/ants/v2"
)

var Dialer = net.Dialer{
	Timeout: time.Minute,
}

var PeerIDPrefix = fmt.Sprintf("-TY%x%x%x0-", MAJOR, MINOR, PATCH)

type pool struct {
	pool *ants.Pool
}

func (p *pool) Submit(task func()) {
	_ = p.pool.Submit(task)
}
