package util

type StrMap = map[string]string

func BitmapLen(n uint32) uint32 {
	if n%8 == 0 {
		return n / 8
	}

	return n/8 + 1
}
