package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type marketplaceConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

type marketplaceModule struct {
	ExecutionRuntimes []string `json:"executionRuntimes,omitempty"`
	Owner             string   `json:"owner"`
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Version           string   `json:"version"`
	SHA256            string   `json:"sha256"`
	Size              int64    `json:"size"`
	Platforms         []string `json:"platforms"`
	Permissions       []string `json:"permissions"`
	SourceURL         string   `json:"sourceUrl"`
}

var marketplaceOwner = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}$`)
var marketplaceDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

var marketplaceBlockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2002::/16"),
}

func marketplaceOrigin(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil {
		return nil, errors.New("Invalid marketplace address")
	}
	invalidParts := u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != ""
	invalidPath := u.Path != "" && u.Path != "/"
	if invalidParts || invalidPath || u.Hostname() == "" || len(value) > 512 {
		return nil, errors.New("Marketplace address must be an origin")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := ip != nil && ip.IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("Marketplace requires HTTPS; HTTP is allowed only on literal loopback")
	}
	if ip != nil && !loopback && !marketplacePublicIP(ip) {
		return nil, errors.New("Private marketplace address is not allowed")
	}
	u.Path = ""
	return u, nil
}

func marketplacePublicIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
		return false
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	for _, prefix := range marketplaceBlockedNetworks {
		if prefix.Contains(address.Unmap()) {
			return false
		}
	}
	return true
}

func (a *app) marketplaceConfiguration() (marketplaceConfig, error) {
	config := marketplaceConfig{}
	raw, err := os.ReadFile(filepath.Join(a.dir, "marketplace.json"))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(raw, &config)
	return config, err
}

// Requests never inherit browser cookies, authentication, proxies or redirects.
// Resolve and validate at dial time, then connect to that exact IP to prevent DNS rebinding.
func marketplaceFetch(ctx context.Context, origin *url.URL, path string, limit int64) ([]byte, error) {
	// Use canonical paths for Next.js trailingSlash deployments without allowing redirects.
	parts := strings.SplitN(path, "?", 2)
	path = strings.TrimRight(parts[0], "/")
	last := path[strings.LastIndex(path, "/")+1:]
	// Next treats the dotted semantic-version segment as a file and removes its slash.
	if !strings.Contains(last, ".") {
		path += "/"
	}
	if len(parts) == 2 {
		path += "?" + parts[1]
	}

	literal := net.ParseIP(origin.Hostname())
	local := literal != nil && literal.IsLoopback()
	transport := &http.Transport{
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		MaxResponseHeaderBytes: 32 * 1024,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, candidate := range addresses {
				allowed := marketplacePublicIP(candidate.IP) || (local && candidate.IP.IsLoopback())
				if !allowed {
					return nil, errors.New("Unsafe marketplace destination")
				}
			}
			if len(addresses) == 0 {
				return nil, errors.New("Marketplace address unavailable")
			}
			dialer := &net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   90 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("Marketplace redirects are not allowed")
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin.String()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("Marketplace version unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("Marketplace response too large")
	}
	return raw, nil
}

func (a *app) marketplaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/marketplace", func(w http.ResponseWriter, r *http.Request) {
		config, err := a.marketplaceConfiguration()
		if err != nil {
			apiError(w, 500, "Could not read marketplace settings")
			return
		}
		writeJSON(w, 200, config)
	})
	mux.HandleFunc("PUT /api/marketplace", func(w http.ResponseWriter, r *http.Request) {
		var config marketplaceConfig
		if err := decodeJSON(w, r, &config); err != nil {
			apiError(w, 400, "Invalid settings")
			return
		}
		if config.Enabled || config.URL != "" {
			origin, err := marketplaceOrigin(config.URL)
			if err != nil {
				apiError(w, 400, err.Error())
				return
			}
			config.URL = origin.String()
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		if err := saveJSON(filepath.Join(a.dir, "marketplace.json"), config); err != nil {
			apiError(w, 500, "Could not save marketplace settings")
			return
		}
		writeJSON(w, 200, config)
	})
	mux.HandleFunc("GET /api/marketplace/catalog", a.marketplaceCatalog)
	mux.HandleFunc("GET /api/marketplace/versions", a.marketplaceCatalog)
	mux.HandleFunc("GET /api/marketplace/updates", a.marketplaceUpdates)
	mux.HandleFunc("POST /api/marketplace/install", a.marketplaceInstall)
}

func (a *app) enabledMarketplace(w http.ResponseWriter) (*url.URL, bool) {
	config, err := a.marketplaceConfiguration()
	if err != nil {
		apiError(w, 500, "Could not read marketplace settings")
		return nil, false
	}
	if !config.Enabled {
		apiError(w, 409, "Marketplace is disabled")
		return nil, false
	}
	origin, err := marketplaceOrigin(config.URL)
	if err != nil {
		apiError(w, 400, err.Error())
		return nil, false
	}
	return origin, true
}

func (a *app) marketplaceCatalog(w http.ResponseWriter, r *http.Request) {
	origin, ok := a.enabledMarketplace(w)
	if !ok {
		return
	}
	query := r.URL.Query()
	path := "/api/v1/catalog"
	if strings.HasSuffix(r.URL.Path, "/versions") {
		if !marketplaceOwner.MatchString(query.Get("owner")) || !validID.MatchString(query.Get("id")) {
			apiError(w, 400, "Invalid module identity")
			return
		}
		path = "/api/v1/modules/" + query.Get("owner") + "/" + query.Get("id") + "?runtime=all"
	} else {
		if len(query.Get("query")) > 256 || len(query.Get("cursor")) > 64 {
			apiError(w, 400, "Invalid catalog query")
			return
		}
		filters := url.Values{}
		filters.Set("runtime", "all")
		for _, key := range []string{"query", "platform", "cursor"} {
			filters.Set(key, query.Get(key))
		}
		path += "?" + filters.Encode()
	}
	raw, err := marketplaceFetch(r.Context(), origin, path, 1024*1024)
	if err != nil || !json.Valid(raw) {
		apiError(w, 502, "Marketplace unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(raw)
}

func (a *app) marketplaceInstall(w http.ResponseWriter, r *http.Request) {
	origin, ok := a.enabledMarketplace(w)
	if !ok {
		return
	}
	var input struct {
		Owner   string `json:"owner"`
		ID      string `json:"id"`
		Version string `json:"version"`
		SHA256  string `json:"sha256"`
		Replace bool   `json:"replace"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		apiError(w, 400, "Invalid installation request")
		return
	}
	validIdentity := marketplaceOwner.MatchString(input.Owner) && validID.MatchString(input.ID)
	validVersion := moduleVersion.MatchString(input.Version) && len(input.Version) <= 48
	if !validIdentity || !validVersion || !marketplaceDigest.MatchString(input.SHA256) {
		apiError(w, 400, "Invalid module identity")
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(2 * time.Minute)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		apiError(w, 500, "Could not set installation deadline")
		return
	}
	path := "/api/v1/modules/" + input.Owner + "/" + input.ID + "/versions/" + input.Version + "?runtime=all"
	raw, err := marketplaceFetch(r.Context(), origin, path, 64*1024)
	var metadata marketplaceModule
	if err != nil || json.Unmarshal(raw, &metadata) != nil {
		apiError(w, 502, "Published version unavailable")
		return
	}
	matchingIdentity := strings.EqualFold(metadata.Owner, input.Owner) && metadata.ID == input.ID
	matchingVersion := metadata.Version == input.Version && metadata.SHA256 == input.SHA256
	if !matchingIdentity || !matchingVersion {
		apiError(w, 409, "Published revision changed")
		return
	}
	if metadata.Size <= 0 || metadata.Size > 68*1024*1024 {
		apiError(w, 400, "Invalid package size")
		return
	}
	data, err := marketplaceFetch(r.Context(), origin, "/api/v1/packages/"+input.SHA256, metadata.Size)
	if err != nil {
		apiError(w, 502, "Package unavailable")
		return
	}
	hash := sha256.Sum256(data)
	if int64(len(data)) != metadata.Size || hex.EncodeToString(hash[:]) != input.SHA256 {
		apiError(w, 409, "Package digest mismatch")
		return
	}
	var manifest moduleManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.ID != input.ID || manifest.Version != input.Version {
		apiError(w, 400, "Package identity mismatch")
		return
	}
	// Recheck opt-in after transfer; never install after the user disables or changes the source.
	current, err := a.marketplaceConfiguration()
	if err != nil || !current.Enabled || current.URL != origin.String() {
		apiError(w, 409, "Marketplace settings changed")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	r.ContentLength = int64(len(data))
	r = r.WithContext(context.WithValue(r.Context(), moduleSourceKey{}, moduleInstallSource{Origin: origin.String(), Owner: metadata.Owner, Adopt: input.Replace}))
	if manifest.SchemaVersion == 2 {
		a.installExtensionPackage(w, r, origin.String())
		return
	}
	a.installModulePackage(w, r, origin.String())
}
