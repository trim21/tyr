package web

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5"
	"github.com/ziflex/lecho/v3"

	"tyr/internal/core"
	"tyr/internal/pkg/global"
	"tyr/internal/util"
	"tyr/internal/web/internal/prof"
	"tyr/internal/web/jsonrpc"
)

//go:embed description.md
var desc string

type jsonRpcRequest struct {
	ID json.RawMessage `json:"id"`
}

func New(c *core.Client, token string, debug bool) http.Handler {
	apiSchema := jsonrpc.OpenAPI{}
	apiSchema.Reflector().SpecEns().Info.Title = "JSON-RPC"
	apiSchema.Reflector().SpecEns().Info.Version = "0.0.1"
	apiSchema.Reflector().SpecEns().Info.WithDescription(desc)
	apiSchema.Reflector().SpecEns().SetAPIKeySecurity("api-key", echo.HeaderAuthorization, openapi.InHeader, "need set api header")

	v := validator.New()

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	h := &jsonrpc.Handler{
		OpenAPI:   &apiSchema,
		Validator: v,
	}

	server := echo.New()
	server.Logger = lecho.From(log.Logger)

	server.Use(middleware.Recover())

	if debug {
		server.Debug = true
		prof.Wrap(server)
	}

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

	server.POST("/json_rpc", echo.WrapHandler(h), auth)

	server.GET("/docs/openapi.json", echo.WrapHandler(h.OpenAPI))
	server.GET("/docs/*", echo.WrapHandler(v5.NewHandlerWithConfig(swgui.Config{
		Title:       apiSchema.Reflector().Spec.Info.Title,
		SwaggerJSON: "/docs/openapi.json",
		BasePath:    "/docs/",
		SettingsUI:  jsonrpc.SwguiSettings(util.StrMap{"layout": "'BaseLayout'"}, "/json_rpc"),
	})))

	server.StaticFS("/", frontendFS)

	if global.Dev {
		lo.Must0(
			os.WriteFile(
				"./internal/web/openapi.json",
				lo.Must(json.MarshalIndent(apiSchema.Reflector().Spec, "", "  ")),
				os.ModePerm,
			),
		)
	}

	return server
}
