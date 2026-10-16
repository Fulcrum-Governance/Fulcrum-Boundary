//go:build !windows

package boundarycli

import (
	"io/fs"
	"syscall"
)

// fileOwnerUID reports the owning uid of info, when the platform exposes it.
func fileOwnerUID(info fs.FileInfo) (int, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}
