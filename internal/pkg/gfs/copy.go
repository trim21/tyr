package gfs

import (
	"context"
	"io"

	"tyr/internal/pkg/gctx"
)

func genericCopy(ctx context.Context, dest io.Writer, src io.Reader, buf []byte) error {
	_, err := io.CopyBuffer(dest, gctx.NewReader(ctx, src), buf)

	return err
}
