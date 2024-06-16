package unsafe

import (
	"unsafe"
)

func Bytes(s string) (b []byte) {
	d := unsafe.StringData(s)
	return unsafe.Slice(d, len(s))
}
