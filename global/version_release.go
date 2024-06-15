//go:build release

package global

import "fmt"

var MAJOR = 0
var MINOR = 0
var PATCH = 0

var Version = fmt.Sprintf("%d.%d.%d", MAJOR, MINOR, PATCH)

var UserAgent = fmt.Sprintf("Tyr/%d.%d.%d (https://github.com/trim21/tyr)", MAJOR, MINOR, PATCH)
