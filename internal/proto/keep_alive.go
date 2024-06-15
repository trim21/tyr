package proto

import (
	"io"
)

var keepAlive = []byte{0, 0, 0, 0}

func SendKeepAlive(conn io.ReadWriter) error {
	_, err := conn.Write(keepAlive)
	return err
}
