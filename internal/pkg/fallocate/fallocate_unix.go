//go:build linux || darwin

package fallocate

import (
	"os"

	"github.com/detailyang/go-fallocate"
)

func Fallocate(file *os.File, offset int64, length int64) error {
	return fallocate.Fallocate(file, offset, length)
}
