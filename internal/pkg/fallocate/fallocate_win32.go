//go:build windows

package fallocate

import (
	"os"
)

func Fallocate(file *os.File, offset int64, length int64) error {
	// go-fallocate write bytes to disk, which is unnecessary
	return file.Truncate(length + offset)
}
