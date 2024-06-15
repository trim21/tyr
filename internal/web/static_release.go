//go:build release

package web

import (
	"embed"
	"io/fs"

	"github.com/labstack/echo/v4"
)

//go:embed frontend
var _static embed.FS

var frontendFS fs.FS = echo.MustSubFS(_static, "frontend")
