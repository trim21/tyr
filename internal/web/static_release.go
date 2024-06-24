//go:build release

package web

import (
	"embed"
	"io/fs"

	"github.com/samber/lo"
)

//go:embed frontend
var _static embed.FS

var frontendFS fs.FS = lo.Must(fs.Sub(_static, "frontend"))
