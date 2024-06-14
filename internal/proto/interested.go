package proto

import (
	"net"
)

func SendInterested(conn net.Conn) error {
	_, err := conn.Write([]byte{0, 0, 0, 1, byte(Interested)})
	return err
}

func SendNotInterested(conn net.Conn) error {
	_, err := conn.Write([]byte{0, 0, 0, 1, byte(NotInterested)})
	return err
}
