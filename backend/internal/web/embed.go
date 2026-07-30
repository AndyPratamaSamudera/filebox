package web

import (
	"embed"
	"io/fs"
)

// staticFiles embeds the built SvelteKit frontend so it ships inside the single
// backend binary. The contents are produced by `make frontend`, which copies
// the SvelteKit build output into static/. A .gitkeep placeholder ensures the
// directory always exists so the embed compiles even before a first build.
//
//go:embed all:static
var staticFiles embed.FS

// StaticFS is the frontend with the embed "static/" prefix stripped, so
// index.html, _app/ and favicon.svg live at the FS root. This lets the
// filesystem middleware serve assets directly and fall back to index.html for
// client-side SPA routes (e.g. /login).
var StaticFS, _ = fs.Sub(staticFiles, "static")
