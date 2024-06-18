package util

import (
	"encoding/binary"

	"github.com/RoaringBitmap/roaring/v2"

	"tyr/internal/pkg/bm"
)

type StrMap = map[string]string

func BitmapFromChunked(b []byte) *bm.Bitmap {
	bmLen := len(b)

	if bmLen%8 != 0 {
		bmLen = (bmLen/8 + 1) * 8
	}

	var bb = make([]uint64, bmLen)
	for i := 0; i < bmLen; i += 8 {
		bb[i] = binary.BigEndian.Uint64(b[i : i+8])
	}

	bitmap := roaring.FromDense(bb, false)

	return bm.FromBitmap(bitmap)
}

func BitmapToChunked(bm *bm.Bitmap, piecesLen int) []byte {
	var b = bm.Bytes()
	if piecesLen&8 == 0 {
		return b[:piecesLen/8]
	}

	return b[:piecesLen/8+1]
}

func BitmapLen(n uint32) uint32 {
	if n%8 == 0 {
		return n / 8
	}

	return 8 * (n/8 + 1)
}
