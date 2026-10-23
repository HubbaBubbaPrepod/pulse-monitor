//go:build !windows && !linux

package main

import "errors"

func availableStorageBytes(path string) (uint64, error) {
	return 0, errors.New("Storage quota checks are unavailable on this OS")
}
