package api

import (
	_ "embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/mobilerpa/mobilerpa-center/server/internal/config"
)

//go:generate go run ../../cmd/openapi-gen

//go:embed generated/openapi.json
var embeddedOpenAPI []byte

var docsAuthConfig = config.Config{
	DocsAuthEnabled:  true,
	DocsAuthUsername: "admin",
	DocsAuthPassword: "123456",
}

func SetDocsAuthConfig(cfg config.Config) {
	docsAuthConfig = cfg
}

const scalarHTMLTemplate = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>MobileRPA Center API Docs</title>
    <style>
      html, body {
        margin: 0;
        padding: 0;
        width: 100%%;
        height: 100%%;
        background: #f5f7fb;
      }
    </style>
  </head>
  <body>
    <script
      id="api-reference"
      data-url="%s"
      data-configuration='{
        "theme": "purple",
        "layout": "modern",
        "showSidebar": true,
        "hideDownloadButton": false
      }'></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`

func scalarDocs() http.HandlerFunc {
	return docsBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		specURL := strings.TrimSpace(r.URL.Query().Get("spec"))
		if specURL == "" {
			specURL = "/openapi.json"
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, scalarHTMLTemplate, specURL)
	})
}

func openAPIDocument() http.HandlerFunc {
	return docsBasicAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(embeddedOpenAPI)
	})
}

func docsBasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !docsAuthConfig.DocsAuthEnabled {
			next.ServeHTTP(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || username != docsAuthConfig.DocsAuthUsername || password != docsAuthConfig.DocsAuthPassword {
			w.Header().Set("WWW-Authenticate", `Basic realm="MobileRPA API Docs"`)
			writeError(w, http.StatusUnauthorized, "docs_auth_required")
			return
		}
		next.ServeHTTP(w, r)
	}
}
