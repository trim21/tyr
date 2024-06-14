package proto

import (
	"encoding/binary"
	"net"
)

var chokeMessage = func() []byte {
	b := binary.BigEndian.AppendUint32(nil, 1)
	b = append(b, byte(Choke))
	return b
}()

func SendChoke(conn net.Conn) error {
	_, err := conn.Write(chokeMessage)
	return err
}

var unchokeMessage = func() []byte {
	b := binary.BigEndian.AppendUint32(nil, 1)
	b = append(b, byte(Unchoke))
	return b
}()

func SendUnchoke(conn net.Conn) error {
	_, err := conn.Write(unchokeMessage)
	return err
}
