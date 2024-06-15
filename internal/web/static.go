//go:build !release

package web

import (
	"os"
)

// FS is for development, so we don't need to restart process
var frontendFS = os.DirFS("internal/web/frontend/")
