package unsafe

import (
	"unsafe"
)

func Bytes(s string) []byte {
	d := unsafe.StringData(s)
	return unsafe.Slice(d, len(s))
}
