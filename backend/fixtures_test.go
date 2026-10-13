package main

import "os"
import "path/filepath"

func newTestApp(dir string) (*app, error) {
	if _, err := os.Stat(filepath.Join(dir, "servers.json")); os.IsNotExist(err) {
		nodes := []server{}
		for _, id := range []string{"production-01", "windows-02", "backup-03"} {
			nodes = append(nodes, server{ID: id, Name: id, Address: "https://" + id + ".example", Source: "waiting", Group: "Test", Interval: 5, Threshold: 85})
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
		if err := saveJSON(filepath.Join(dir, "servers.json"), nodes); err != nil {
			return nil, err
		}
	}
	return newApp(dir)
}
