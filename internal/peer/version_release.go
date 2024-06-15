//go:build release

package peer

import "fmt"

var MAJOR = 0
var MINOR = 0
var PATCH = 0

var UserAgent = fmt.Sprintf("VE/%d.%d.%d", MAJOR, MINOR, PATCH)
var peerIDPrefix = fmt.Sprintf("-VE%x%x%x0-", MAJOR, MINOR, PATCH)
