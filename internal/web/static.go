package web

import (
	_ "embed"
	"net/http"
)

//go:embed assets/index.html
var indexHTML []byte

//go:embed assets/app.css
var appCSS []byte

//go:embed assets/app.js
var appJS []byte

func (a *API) IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}
func (a *API) CSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(appCSS)
}
func (a *API) JSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(appJS)
}
