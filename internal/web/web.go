package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5"

	"ve/internal/client"
	"ve/internal/web/jsonrpc"
)

func New(c *client.Client) http.Handler {
	apiSchema := jsonrpc.OpenAPI{}
	apiSchema.Reflector().SpecEns().Info.Title = "JSON-RPC"
	apiSchema.Reflector().SpecEns().Info.Version = "v0.0.1"
	apiSchema.Reflector().SpecEns().Info.WithDescription("JSON API")

	h := &jsonrpc.Handler{
		OpenAPI:              &apiSchema,
		Validator:            &jsonrpc.JSONSchemaValidator{},
		SkipResultValidation: true,
	}

	r := echo.New()

	AddTorrent(h, c)

	r.POST("/json_rpc", echo.WrapHandler(h))

	r.GET("/docs/openapi.json", echo.WrapHandler(h.OpenAPI))
	r.GET("/docs/*", echo.WrapHandler(v5.NewHandlerWithConfig(swgui.Config{
		Title:       apiSchema.Reflector().Spec.Info.Title,
		SwaggerJSON: "/docs/openapi.json",
		BasePath:    "/docs/",
		SettingsUI:  jsonrpc.SwguiSettings(nil, "/json_rpc"),
	})))

	r.StaticFS("/", frontendFS)

	return r
}
