package global

import (
	"fmt"
	"net"
)

var Dialer net.Dialer

var PeerIDPrefix = fmt.Sprintf("-TY%x%x%x0-", MAJOR, MINOR, PATCH)
