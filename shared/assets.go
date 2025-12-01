package shared

import (
	"embed"
	"net/http"
)

//go:embed *.js
//go:embed *.css
var content embed.FS

// FS provides a filesystem of static assets
// that are embedded into the production binary.
var FS = http.FS(content)
