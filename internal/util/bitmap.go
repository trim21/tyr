package util

import "github.com/kelindar/bitmap"

func BitmapFromChunked(b []byte) bitmap.Bitmap {
	bmLen := len(b)

	if bmLen%8 != 0 {
		bmLen = (bmLen/8 + 1) * 8
	}

	var bb = make([]byte, bmLen)
	copy(bb, b)

	return bitmap.FromBytes(bb)
}

func BitmapToChunked(bm bitmap.Bitmap, piecesLen int) []byte {
	var b = bm.ToBytes()
	if piecesLen&8 == 0 {
		return b[:piecesLen/8]
	}

	return b[:piecesLen/8+1]
}
