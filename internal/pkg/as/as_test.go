//go:build !release

package as_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tyr/internal/pkg/as"
)

func TestUint32(t *testing.T) {
	require.Equal(t, uint32(5), as.Uint32(int8(5)))
	require.Equal(t, uint32(5), as.Uint32(int16(5)))
	require.Equal(t, uint32(5), as.Uint32(int32(5)))
	require.Equal(t, uint32(5), as.Uint32(int64(5)))
	require.Equal(t, uint32(5), as.Uint32(int(5)))
	require.Equal(t, uint32(5), as.Uint32(uint8(5)))
	require.Equal(t, uint32(5), as.Uint32(uint16(5)))
	require.Equal(t, uint32(5), as.Uint32(uint64(5)))
	require.Equal(t, uint32(5), as.Uint32(uint(5)))

	assert.Panics(t, func() {
		as.Uint32(math.MaxUint32 + 1)
	})
}
