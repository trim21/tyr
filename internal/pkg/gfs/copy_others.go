//go:build !unix

package gfs

import (
	"context"
	"os"

	"tyr/internal/pkg/flowrate"
)

func fileCopy(ctx context.Context, dest *os.File, src *os.File, buf []byte, monitor *flowrate.Monitor) error {
	return genericCopy(ctx, dest, monitor.WrapReader(src), buf)
}
