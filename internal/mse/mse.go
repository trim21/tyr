package mse

import (
	"io"
	"net"

	"github.com/anacrolix/torrent/mse"

	"tyr/internal/pkg/unsafe"
)

type rw struct {
	io.Reader
	io.Writer
}

func ForceCrypto(provided mse.CryptoMethod) mse.CryptoMethod {
	// We prefer plaintext for performance reasons.
	if provided&mse.CryptoMethodRC4 != 0 {
		return mse.CryptoMethodRC4
	}
	return mse.CryptoMethodPlaintext
}

func PreferCrypto(provided mse.CryptoMethod) mse.CryptoMethod {
	// We prefer plaintext for performance reasons.
	if provided&mse.CryptoMethodRC4 != 0 {
		return mse.CryptoMethodRC4
	}
	return mse.CryptoMethodPlaintext
}

func NewAccept(conn net.Conn, keys []string, selector mse.CryptoSelector) (io.ReadWriteCloser, error) {
	rwc, _, err := mse.ReceiveHandshake(conn, func(f func([]byte) bool) {
		for _, ih := range keys {
			if !f(unsafe.Bytes(ih)) {
				break
			}
		}
	}, selector)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return wrappedConn{ReadWriter: rwc, Closer: conn}, err
}

func NewConnection(infoHash []byte, conn net.Conn) (io.ReadWriteCloser, error) {
	ret, _, err := mse.InitiateHandshake(conn, infoHash, nil, mse.AllSupportedCrypto)
	if err != nil {
		return nil, err
	}

	return wrappedConn{ReadWriter: ret, Closer: conn}, nil
}

var _ io.ReadWriteCloser = wrappedConn{}

type wrappedConn struct {
	io.ReadWriter
	io.Closer
}
