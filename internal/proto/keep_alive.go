package proto

import "net"

var keepAlive = []byte{0, 0, 0, 0}

func SendKeepAlive(conn net.Conn) error {
	_, err := conn.Write(keepAlive)
	return err
}
