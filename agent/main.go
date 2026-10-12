package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

func backendURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("url must be the panel origin without credentials or path")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errors.New("remote panel requires HTTPS")
	}
	return strings.TrimRight(raw, "/"), nil
}
func atomicFile(path string, value []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".pulse-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	_, writeErr := f.Write(value)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func deliver(ctx context.Context, client *http.Client, endpoint, token string, raw []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode != 200 {
		return fmt.Errorf("panel returned HTTP %d", response.StatusCode)
	}
	return nil
}
func run(parent context.Context) error {
	panel := flag.String("url", "http://127.0.0.1:8080", "Panel HTTPS origin (loopback HTTP allowed)")
	id := flag.String("server", "", "Server ID from panel")
	tokenFile := flag.String("token-file", "agent.token", "File containing the node token")
	state := flag.String("state", "agent-state", "Persistent retry queue directory")
	interval := flag.Duration("interval", 500*time.Millisecond, "Sampling interval, 500ms to 5s")
	capture := flag.String("capture", "", "Optional TShark interface name or number; empty disables packet capture")
	check := flag.Bool("check", false, "Collect and print a diagnostic summary without connecting")
	flag.Parse()
	if *check {
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)
		defer cancel()
		report, err := newCollector().collect(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"os": report.OS, "hardware": report.Hardware, "capabilities": report.Capabilities, "processes": len(report.Processes), "disks": len(report.Disks), "connections": len(report.Connections), "gpus": report.GPUs})
	}
	base, err := backendURL(*panel)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`).MatchString(*id) {
		return errors.New("valid -server ID is required")
	}
	if *interval < 500*time.Millisecond || *interval > 5*time.Second {
		return errors.New("interval must be between 500ms and 5s")
	}
	raw, err := os.ReadFile(*tokenFile)
	if err != nil {
		return fmt.Errorf("read token file: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if _, err := hex.DecodeString(token); err != nil || len(token) != 64 {
		return errors.New("invalid token file")
	}
	if err := os.MkdirAll(*state, 0700); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	transport.MaxIdleConnsPerHost = 2
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	defer transport.CloseIdleConnections()
	var bootRaw [16]byte
	if _, err := rand.Read(bootRaw[:]); err != nil {
		return err
	}
	boot := hex.EncodeToString(bootRaw[:])
	sequence := uint64(0)
	c := newCollector()
	cursorPath := filepath.Join(*state, "log-cursor")
	if raw, err := os.ReadFile(cursorPath); err == nil {
		stamp, err := time.Parse(time.RFC3339Nano, string(raw))
		if err != nil {
			return err
		}
		if stamp.Before(time.Now()) {
			c.logCursor = stamp
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	packets, finishCapture := startCapture(ctx, *capture)
	defer finishCapture()
	pendingPath := filepath.Join(*state, "pending.json")
	pending, err := os.ReadFile(pendingPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	retry := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		if len(pending) == 0 {
			collectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			report, err := c.collect(collectCtx)
			cancel()
			if err != nil {
				slog.Warn("collection failed", "error", err)
				if !wait(ctx, time.Second) {
					break
				}
				continue
			}
			sequence++
			report.BootID = boot
			report.Sequence = sequence
			report.LogCursor = c.logCursor.UTC().Format(time.RFC3339Nano)
			report.Packets = packets.snapshot()
			report.Capabilities["packets"] = packets.status()
			if sequence == 1 && len(report.Logs) < 100 {
				report.Logs = append(report.Logs, logEntry{Time: time.Now().UTC().Format(time.RFC3339Nano), Level: "INFO", Source: "pulse-agent", Message: "Agent started on " + report.Hardware.Hostname})
			}
			pending, err = json.Marshal(report)
			if err != nil {
				return err
			}
			if err := atomicFile(pendingPath, pending); err != nil {
				return err
			}
		}
		if err := deliver(ctx, client, base+"/api/agent/"+*id+"/report", token, pending); err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Warn("report queued; retrying", "error", err, "retry", retry)
			if !wait(ctx, retry) {
				break
			}
			retry = min(30*time.Second, retry*2)
			continue
		}
		var delivered agentReport
		if err := json.Unmarshal(pending, &delivered); err != nil {
			return err
		}
		if delivered.LogCursor != "" {
			if err := atomicFile(cursorPath, []byte(delivered.LogCursor)); err != nil {
				return err
			}
			if stamp, err := time.Parse(time.RFC3339Nano, delivered.LogCursor); err == nil {
				c.logCursor = stamp
			}
		}
		if err := os.Remove(pendingPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pending = nil
		retry = time.Second
		if !wait(ctx, max(0, *interval-time.Since(start))) {
			break
		}
	}
	return nil
}
func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func main() {
	if err := platformRun(); err != nil {
		slog.Error("Pulse agent stopped", "error", err)
		os.Exit(1)
	}
}
