//go:build embedweb

package webapp

import (
	"embed"
	"io/fs"
)

// dist is written by `npm run build` in web/ (see web/vite.config.ts) — build
// the frontend before compiling with this tag.
//
//go:embed all:dist
var dist embed.FS

func assets() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}

	return sub, true
}
