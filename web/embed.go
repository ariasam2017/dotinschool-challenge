// Package web embeds the built Vite frontend so the final binary is a single
// self-contained file with no external asset dependency.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:assets
var embedded embed.FS

// Assets is the built frontend rooted at what Vite emits into web/assets.
// During development this directory may hold only a placeholder; the real
// build overwrites it before `go build`.
func Assets() fs.FS {
	sub, err := fs.Sub(embedded, "assets")
	if err != nil {
		// The //go:embed directive guarantees "assets" exists at build time.
		panic(err)
	}
	return sub
}
