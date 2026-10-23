package main

import "golang.org/x/sys/windows"

func availableStorageBytes(path string) (uint64, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free, total, unused uint64
	err = windows.GetDiskFreeSpaceEx(name, &free, &total, &unused)
	return free, err
}
