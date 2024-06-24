//go:build release

package as

func Uint32[T int8 | int16 | int32 | int64 | int | uint8 | uint16 | uint64 | uint](v T) uint32 {
	return uint32(v)
}
