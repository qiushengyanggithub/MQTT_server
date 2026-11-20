//go:build !embedweb

package mqtt

import "io/fs"

// WebDistFS returns nil when the binary was built without the embedweb tag.
func WebDistFS() fs.FS { return nil }
