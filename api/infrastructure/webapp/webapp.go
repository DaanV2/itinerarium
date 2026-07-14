// Package webapp embeds the production build of the SvelteKit frontend so a
// single server binary ships the whole application. The site is baked in only
// when compiling with the "embedweb" build tag (`just build`, the Dockerfile);
// without it Assets reports false and the server runs API-only — which is what
// dev builds and CI's plain `go build ./...` do, so the Go toolchain never
// requires a web build.
package webapp

import "io/fs"

// Assets returns the built frontend rooted at the site root (index.html at
// the top). ok is false when the binary was compiled without the "embedweb"
// build tag.
func Assets() (fs.FS, bool) {
	return assets()
}
