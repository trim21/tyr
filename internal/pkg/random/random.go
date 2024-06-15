package random

import (
	"bufio"
	"crypto/rand"
	"io"

	"tyr/internal/pkg/pool"
)

var p = pool.New(func() *bufio.Reader {
	return bufio.NewReader(rand.Reader)
})

// we may never need to change these values.
const base64Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// Base64Str generate a cryptographically secure base64 string in given length.
// Will panic if it can't read from 'crypto/rand'.
func Base64Str(length int) string {
	reader := p.Get()
	defer p.Put(reader)

	r := make([]byte, length) //nolint:gomnd
	i := 0

	for {
		n, err := io.ReadFull(reader, r)
		if err != nil {
			panic("unexpected error happened when reading from bufio.NewReader(crypto/rand.Reader)")
		}
		if n != len(r) {
			panic("partial reads occurred when reading from bufio.NewReader(crypto/rand.Reader)")
		}
		for _, rb := range r {
			r[i] = base64Chars[rb%64]
			i++
			if i == length {
				return string(r)
			}
		}
	}
}
