//go:build !release

package global

var MAJOR = 0
var MINOR = 0
var PATCH = 0

var UserAgent = "Tyr/development (https://github.com/trim21/tyr)"

var Dev bool = true

// write to `Dev` so some analyzer won't think Dev is always true in dev mode
func init() {
	Dev = true
}
