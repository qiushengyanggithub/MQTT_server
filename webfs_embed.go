//go:build embedweb

package mqtt

import (
	"embed"
	"io/fs"
)

// webDistFS contains the Vue3 dashboard build output compiled into the binary.
//
//go:embed web/dist
var webDistFS embed.FS

// WebDistFS returns the embedded web/dist filesystem, rooted at the dist directory.
func WebDistFS() fs.FS {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
