package proto_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"tyr/internal/proto"
)

func TestSendPiece(t *testing.T) {
	var b bytes.Buffer

	assert.NoError(t, proto.SendPiece(&b, 20, 5, []byte("hello world")))

	assert.Equal(t, "\x00\x00\x00\x14\x07\x00\x00\x00\x14\x00\x00\x00\x05hello world", b.String())
}
