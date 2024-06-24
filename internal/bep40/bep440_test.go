//go:build bep40_spec

package bep40_test

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"tyr/internal/bep40"
	"tyr/internal/pkg/crc32c"
)

func TestBep40Priority4(t *testing.T) {
	require.EqualValues(t, "\x00\x00\x00\x00", bep40.PriorityBytes4(
		netip.MustParseAddrPort("123.213.32.234:0"),
		netip.MustParseAddrPort("123.213.32.234:0"),
	))

	require.EqualValues(t, crc32c.Sum(lo.Must(hex.DecodeString("624C14007BD50000"))), 0xec2d7224)
	require.EqualValues(t, crc32c.Sum(lo.Must(hex.DecodeString("7BD5200A7BD520EA"))), 0x99568189)

	require.EqualValues(t, lo.Must(hex.DecodeString("624C14007BD50000")), bep40.PriorityBytes4(
		netip.MustParseAddrPort("123.213.32.10:0"),
		netip.MustParseAddrPort("98.76.54.32:0"),
	))

	require.EqualValues(t, lo.Must(hex.DecodeString("7BD5200A7BD520EA")), bep40.PriorityBytes4(
		netip.MustParseAddrPort("123.213.32.10:0"),
		netip.MustParseAddrPort("123.213.32.234:0"),
	))

	require.EqualValues(t, 0xec2d7224, bep40.Priority4(
		netip.MustParseAddrPort("123.213.32.10:0"),
		netip.MustParseAddrPort("98.76.54.32:0"),
	))

	require.EqualValues(t, 0xec2d7224, bep40.Priority4(
		netip.MustParseAddrPort("98.76.54.32:0"),
		netip.MustParseAddrPort("123.213.32.10:0"),
	))
	require.EqualValues(t, 0x99568189, bep40.Priority4(
		netip.MustParseAddrPort("123.213.32.10:0"),
		netip.MustParseAddrPort("123.213.32.234:0"),
	))

	require.EqualValues(t, 0x2b41d456, bep40.Priority4(
		netip.MustParseAddrPort("206.248.98.111:0"),
		netip.MustParseAddrPort("142.147.89.224:0"),
	))
}

func TestV6(t *testing.T) {
	require.EqualValues(t, "\x00\x00\x00\x01", bep40.PriorityBytes6(
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:483e:800a]:1"),
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:483e:800a]:0"),
	))

	require.EqualValues(t, lo.Must(hex.DecodeString("201576936cd9a56ae47f710100000000"+"201576936cd9a56ae47f7101ffff5555")), bep40.PriorityBytes6(
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:ffff:ffff]:0"),
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:0000:0000]:0"),
	))

	require.EqualValues(t, lo.Must(hex.DecodeString("201576936cd9a56ae47f7101483e0b0a"+"201576936cd9a56ae47f7101483e800a")), bep40.PriorityBytes6(
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:483e:800a]:0"),
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:483e:0b0a]:0"),
	))

	require.EqualValues(t, uint32(0xfbd26e29), bep40.Priority6(
		netip.MustParseAddrPort("[2015:7693:6cd9:a56a:e47f:7101:483e:800a]:0"),
		netip.MustParseAddrPort("[b1fa:9ff2:fbdc:23b9:3618:332c:216c:5b4a]:0"),
	))
}
