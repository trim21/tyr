package web

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5"

	"tyr/internal/client"
	"tyr/internal/web/jsonrpc"
)

//go:embed description.md
var desc string

type jsonRpcRequest struct {
	ID json.RawMessage `json:"id"`
}

func New(c *client.Client, token string) http.Handler {
	apiSchema := jsonrpc.OpenAPI{}
	apiSchema.Reflector().SpecEns().Info.Title = "JSON-RPC"
	apiSchema.Reflector().SpecEns().Info.Version = "0.0.1"
	apiSchema.Reflector().SpecEns().Info.WithDescription(desc)

	h := &jsonrpc.Handler{
		OpenAPI:              &apiSchema,
		Validator:            &jsonrpc.JSONSchemaValidator{},
		SkipResultValidation: true,
	}

	r := echo.New()

	AddTorrent(h, c)

	var auth echo.MiddlewareFunc = func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get(echo.HeaderAuthorization) != token {
				var r jsonRpcRequest
				err := json.NewDecoder(c.Request().Body).Decode(&r)
				if err != nil {
					return c.JSON(401,
						jsonrpc.Response{
							JSONRPC: "2.0",
							Result:  nil,
							Error: &jsonrpc.Error{
								Code:    jsonrpc.CodeParseError,
								Message: err.Error(),
							},
						},
					)
				}
				return c.JSON(401, jsonrpc.Response{
					JSONRPC: "2.0",
					ID:      r.ID,
					Error:   &jsonrpc.Error{Code: 401, Message: "invalid token"},
				})
			}

			return next(c)
		}
	}

	r.POST("/json_rpc", echo.WrapHandler(h), auth)

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
