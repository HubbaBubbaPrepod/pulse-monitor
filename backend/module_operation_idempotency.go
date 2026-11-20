package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

func validModuleRequestKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, c := range key {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.:", c) {
			return false
		}
	}
	return true
}

func (a *app) manualRequestIdentity(key string) (string, string) {
	a.auth.mu.Lock()
	defer a.auth.mu.Unlock()
	owner := ""
	if a.auth.account != nil {
		owner = a.auth.account.ID
	}
	return "manual-request:" + owner + ":" + key, owner
}

func (a *app) queueManualModuleOperation(p modulePackageV2, c moduleComponentV2, server, action string, input map[string]any, key string) (moduleOperation, int, error) {
	if key == "" {
		return a.queueModuleOperation(p, c, server, "manual", action, input, nil)
	}
	if !validModuleRequestKey(key) {
		return moduleOperation{}, 400, errors.New("Invalid idempotency key")
	}
	identity, owner := a.manualRequestIdentity(key)
	cfg, err := a.extensionConfiguration(p.ID, c.ID, server)
	if err != nil {
		return moduleOperation{}, 403, errors.New("Component configuration unavailable")
	}
	raw, err := json.Marshal(struct {
		Owner, Module, Component, Version, Server, Action, Configuration string
		Input                                                            map[string]any
	}{owner, p.ID, c.ID, p.Version, server, action, moduleConfigurationDigest(cfg), input})
	if err != nil {
		return moduleOperation{}, 400, errors.New("Invalid action input")
	}
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	id := sha256.Sum256([]byte(identity))
	old, err := a.moduleOperation(hex.EncodeToString(id[:]))
	if err == nil {
		if old.RequestDigest != fingerprint {
			return moduleOperation{}, 409, errors.New("Idempotency key is already bound to another request or configuration")
		}
		if _, _, _, err = a.authorizedOperation(old); err != nil {
			return moduleOperation{}, 403, err
		}
		return old, 202, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return moduleOperation{}, 500, err
	}
	return a.queueModuleOperation(p, c, server, "manual", action, input, nil, identity, fingerprint)
}

func (a *app) moduleRequestStatus(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if !validModuleRequestKey(key) {
		apiError(w, 400, "Invalid idempotency key")
		return
	}
	identity, owner := a.manualRequestIdentity(key)
	id := sha256.Sum256([]byte(identity))
	a.extensionMu.Lock()
	defer a.extensionMu.Unlock()
	op, err := a.moduleOperation(hex.EncodeToString(id[:]))
	if err != nil || op.RequestedBy != owner || op.RequestDigest == "" {
		apiError(w, 404, "Operation not found")
		return
	}
	writeJSON(w, 200, publicModuleOperation(op))
}
