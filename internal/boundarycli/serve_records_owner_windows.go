//go:build windows

package boundarycli

import "io/fs"

// fileOwnerUID has no POSIX uid to report on Windows; ownership is not checked
// there and access is governed by the directory's ACL.
func fileOwnerUID(fs.FileInfo) (int, bool) { return 0, false }
