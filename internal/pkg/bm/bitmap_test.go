package bm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tyr/internal/pkg/bm"
)

func TestBitmap(t *testing.T) {
	b := bm.New(10)
	b.Fill()
	require.True(t, b.Get(9))
	require.False(t, b.Get(10))
}
