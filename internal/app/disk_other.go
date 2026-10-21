//go:build !linux && !darwin

package app

import "errors"

func diskCapacity(path string) (uint64, uint64, error) {
	return 0, 0, errors.New("disk capacity is unavailable on this OS")
}
