package ui

import (
	"io/fs"
	"net/http"

	"github.com/bapley/tld-redirect/web"
)

func Handler() http.Handler {
	sub, _ := fs.Sub(web.StaticFS, "static")
	return http.StripPrefix("/ui/", http.FileServer(http.FS(sub)))
}
