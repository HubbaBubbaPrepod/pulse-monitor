package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
)

// Remove explicitly labelled demo data once, retaining pre-migration backups.
func (a *app) migrateDemo() error {
	old := slices.Clone(a.servers)
	keptServers := make([]server, 0, len(old))
	changed := false
	for _, s := range old {
		if s.Source == "demo" {
			raw, err := os.ReadFile(a.nodePath(s.ID))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			var node agentNode
			if err == nil {
				if err := json.Unmarshal(raw, &node); err != nil {
					return err
				}
			}
			changed = true
			if node.Hash == "" && node.Report.OS == "" {
				continue
			}
			s.Source = "waiting"
			s.Online = false
			s.CPU, s.Memory, s.Disk = 0, 0, 0
		}
		keptServers = append(keptServers, s)
	}
	a.servers = keptServers
	if changed {
		backup := filepath.Join(a.dir, "servers.pre-real.json")
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			if err := saveJSON(backup, old); err != nil {
				return err
			}
		}
		if err := a.persistServers(a.servers); err != nil {
			return err
		}
	}
	path := filepath.Join(a.dir, "logs.jsonl")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(a.dir, ".logs-*")
	if err != nil {
		f.Close()
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	defer temp.Close()
	defer f.Close()
	reader := bufio.NewReader(f)
	removed := false
	for {
		raw, readErr := reader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		var l logEntry
		if json.Unmarshal(raw, &l) == nil && l.DataSource == "demo" {
			removed = true
		} else if _, err := temp.Write(raw); err != nil {
			return err
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}
	closeErr := f.Close()
	if closeErr != nil {
		return closeErr
	}
	if !removed {
		return nil
	}
	backup := filepath.Join(a.dir, "logs.pre-real.jsonl")
	if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		copyFile, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(copyFile, source)
		backupErr := errors.Join(copyErr, copyFile.Sync(), copyFile.Close(), source.Close())
		if backupErr != nil {
			os.Remove(backup)
			return backupErr
		}
	} else if err != nil {
		return err
	}
	syncErr := temp.Sync()
	closeErr = temp.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(name, path)
}
