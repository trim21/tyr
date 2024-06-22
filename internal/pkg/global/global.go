package global

import (
	"fmt"
	"net"
	"time"
)

var Dialer = net.Dialer{
	Timeout: time.Minute,
}

var PeerIDPrefix = fmt.Sprintf("-TY%x%x%x0-", MAJOR, MINOR, PATCH)
