//go:build linux || darwin

package app

import "golang.org/x/sys/unix"

func diskCapacity(path string) (total, available uint64, err error) {
	var s unix.Statfs_t
	if err = unix.Statfs(path, &s); err != nil {
		return
	}
	total = uint64(s.Blocks) * uint64(s.Bsize)
	available = uint64(s.Bavail) * uint64(s.Bsize)
	return
}
