package random

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"

	"tyr/internal/pkg/pool"
)

var p = pool.New(func() *bufio.Reader {
	return bufio.NewReader(rand.Reader)
})

const base64Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// Base64Str generate a cryptographically secure base64 string in given length.
// Will panic if it can't read from 'crypto/rand'.
func Base64Str(size int) string {
	r := Bytes(size)

	for i, rb := range r {
		// len(base64Chars) % 64 == 0 so it's not biaed
		r[i] = base64Chars[rb%64]
	}

	return string(r)
}

// Bytes generate a cryptographically secure random bytes.
// Will panic if it can't read from 'crypto/rand'.
func Bytes(size int) []byte {
	reader := p.Get()
	defer p.Put(reader)

	r := make([]byte, size)
	_, err := io.ReadFull(reader, r)
	if err != nil {
		panic(fmt.Sprintf("unexpected error happened when reading from bufio.NewReader(crypto/rand.Reader) %+v", err))
	}

	return r
}
