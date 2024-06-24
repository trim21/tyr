package web

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/samber/lo"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/swgui"
	v5 "github.com/swaggest/swgui/v5"

	"tyr/internal/core"
	"tyr/internal/pkg/global"
	"tyr/internal/util"
	"tyr/internal/web/jsonrpc"
)

//go:embed description.md
var desc string

type jsonRpcRequest struct {
	ID json.RawMessage `json:"id"`
}

const HeaderAuthorization = "Authorization"

func New(c *core.Client, token string, debug bool) http.Handler {
	apiSchema := jsonrpc.OpenAPI{}
	apiSchema.Reflector().SpecEns().Info.Title = "JSON-RPC"
	apiSchema.Reflector().SpecEns().Info.Version = "0.0.1"
	apiSchema.Reflector().SpecEns().Info.WithDescription(desc)
	apiSchema.Reflector().SpecEns().SetAPIKeySecurity("api-key", HeaderAuthorization, openapi.InHeader, "need set api header")

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

	r := chi.NewMux()
	r.Use(middleware.Recoverer)
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("."))
		return
	})

	if debug {
		r.Mount("/debug", middleware.Profiler())
	}

	AddTorrent(h, c)

	var auth = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(HeaderAuthorization) != token {
				w.WriteHeader(401)
				_ = json.NewEncoder(w).Encode(jsonrpc.Response{
					JSONRPC: "2.0",
					Error: &jsonrpc.Error{
						Code:    jsonrpc.CodeInvalidRequest,
						Message: "invalid token",
					},
				})

				return
			}

			next.ServeHTTP(w, r)

			return
		})
	}

	r.With(auth).Post("/json_rpc", h.ServeHTTP)
	r.Get("/docs/openapi.json", h.OpenAPI.ServeHTTP)
	r.Get("/docs/*", v5.NewHandlerWithConfig(swgui.Config{
		Title:       apiSchema.Reflector().Spec.Info.Title,
		SwaggerJSON: "/docs/openapi.json",
		BasePath:    "/docs/",
		SettingsUI:  jsonrpc.SwguiSettings(util.StrMap{"layout": "'BaseLayout'"}, "/json_rpc"),
	}).ServeHTTP)

	r.Handle("/*", http.FileServerFS(frontendFS))

	if global.Dev {
		lo.Must0(
			os.WriteFile(
				"./internal/web/openapi.json",
				lo.Must(json.MarshalIndent(apiSchema.Reflector().Spec, "", "  ")),
				os.ModePerm,
			),
		)
	}

	return r
}
