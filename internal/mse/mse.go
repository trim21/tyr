package mse

import (
	"io"
	"net"

	"github.com/anacrolix/torrent/mse"
)

type rw struct {
	io.Reader
	io.Writer
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
