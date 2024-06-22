package prof

import (
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
)

// Wrap adds several routes from package `net/http/pprof` to *echo.Echo object.
func Wrap(e *echo.Echo) {
	e.GET("/debug/pprof", func(c echo.Context) error {
		return c.Redirect(302, "/debug/pprof/")
	})

	e.GET("/debug/pprof/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))

	for _, p := range []string{"heap", "allocs", "goroutine", "block", "mutex", "threadcreate"} {
		e.GET("/debug/pprof/"+p, echo.WrapHandler(pprof.Handler(p)))
	}

	e.GET("/debug/pprof/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	e.GET("/debug/pprof/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	e.GET("/debug/pprof/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	e.Any("/debug/pprof/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
}
