package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func run() error {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address (development loopback only)")
	dataDir := flag.String("data", "data", "Directory for persistent logs and server definitions")
	staticDir := flag.String("static", "out", "Next.js static export directory")
	tlsCert := flag.String("tls-cert", "", "PEM certificate for HTTPS")
	tlsKey := flag.String("tls-key", "", "PEM private key for HTTPS")
	publicURL := flag.String("public-url", "", "Canonical HTTPS URL for remote deployment")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	remote := host != "localhost" && (ip == nil || !ip.IsLoopback())
	if (*tlsCert == "") != (*tlsKey == "") {
		return errors.New("both TLS certificate and key are required")
	}
	var canonical *url.URL
	if *publicURL != "" {
		canonical, err = url.Parse(*publicURL)
		if err != nil || canonical.Scheme != "https" || canonical.Host == "" || canonical.User != nil || canonical.RawQuery != "" || canonical.Fragment != "" || (canonical.Path != "" && canonical.Path != "/") {
			return errors.New("public-url must be a canonical HTTPS origin")
		}
	}
	if remote && (*tlsCert == "" || canonical == nil) {
		return errors.New("remote listening requires tls-cert, tls-key and public-url")
	}
	a, err := newApp(*dataDir)
	if err != nil {
		return err
	}
	if remote && a.auth.account == nil {
		return errors.New("create the owner account on loopback before enabling remote access")
	}
	if canonical != nil {
		a.publicHost = canonical.Host
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mux := http.NewServeMux()
	mux.Handle("/api/", a.handler())
	root, err := filepath.Abs(*staticDir)
	if err != nil {
		return err
	}
	files := http.FileServer(http.Dir(root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Never expose directory listings or dotfiles from the export tree.
		for _, part := range strings.Split(r.URL.Path, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		} else if strings.HasSuffix(clean, "/") {
			clean += "index.html"
		}
		if !filepath.IsLocal(filepath.FromSlash(clean)) {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean)))
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	errCh := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); errCh <- a.collect(ctx) }()
	go func() {
		defer workers.Done()
		if *tlsCert != "" {
			errCh <- srv.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()
	scheme := "http"
	if *tlsCert != "" {
		scheme = "https"
	}
	slog.Info("Pulse backend started", "url", scheme+"://"+*addr, "data", *dataDir)
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, srv.Close())
	}
	workers.Wait()
	if errors.Is(runErr, http.ErrServerClosed) {
		runErr = nil
	}
	return errors.Join(runErr, shutdownErr)
}
func main() {
	if err := run(); err != nil {
		slog.Error("Pulse stopped", "error", fmt.Sprint(err))
		os.Exit(1)
	}
}
