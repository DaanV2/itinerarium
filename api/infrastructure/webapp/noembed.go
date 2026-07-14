//go:build !embedweb

package webapp

import "io/fs"

func assets() (fs.FS, bool) {
	return nil, false
}
