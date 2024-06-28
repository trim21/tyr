//go:build windows

package fallocate

import (
	"os"
)

func Fallocate(file *os.File, offset int64, length int64) error {
	if length == 0 {
		return nil
	}

	return file.Truncate(length + offset)
}
