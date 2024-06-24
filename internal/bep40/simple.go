package bep40

import (
	"encoding/binary"
	"net/netip"

	"tyr/internal/pkg/crc32c"
)

func SimplePriority(key []byte, addr []byte) uint32 {
	var bs = make([]byte, len(key)+len(addr))

	copy(bs, key)
	copy(bs[len(key):], addr)

	return crc32c.Sum(bs)
}

func SimplePriority4(key []byte, peer netip.AddrPort) uint32 {
	var bs = make([]byte, 0, len(key)+6)

	bs = append(bs, key...)
	bs = append(bs, peer.Addr().AsSlice()...)
	bs = binary.BigEndian.AppendUint16(bs, peer.Port())

	return crc32c.Sum(bs)
}

func SimplePriority6(key []byte, peer netip.AddrPort) uint32 {
	var bs = make([]byte, 0, len(key)+18)

	bs = append(bs, key...)
	bs = append(bs, peer.Addr().AsSlice()...)
	bs = binary.BigEndian.AppendUint16(bs, peer.Port())

	return crc32c.Sum(bs)
}
