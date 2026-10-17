package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const modulePackageBodyLimit = 68 * 1024 * 1024

func (a *app) modulePreflightRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/module-capabilities", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"schemaVersions": []int{1, 2}, "installableSchemaVersions": []int{1}, "runtimes": []string{}})
	})
	mux.HandleFunc("POST /api/module-packages/validate", func(w http.ResponseWriter, r *http.Request) {
		fail := func(status int, code string) {
			writeJSON(w, status, map[string]any{"valid": false, "compatible": false, "issues": []moduleValidationIssue{{"package", code}}, "compatibilityIssues": []moduleValidationIssue{}})
		}
		if r.ContentLength > modulePackageBodyLimit {
			fail(413, "package_too_large")
			return
		}
		controller := http.NewResponseController(w)
		deadline := time.Now().Add(2 * time.Minute)
		if err := controller.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			fail(500, "read_deadline_unavailable")
			return
		}
		if err := controller.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			fail(500, "write_deadline_unavailable")
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, modulePackageBodyLimit))
		if err != nil {
			var size *http.MaxBytesError
			if errors.As(err, &size) {
				fail(413, "package_too_large")
			} else {
				fail(400, "invalid_package")
			}
			return
		}
		pkg, err := decodeModulePackageV2(raw)
		if err != nil {
			fail(400, "invalid_package")
			return
		}
		issues := validateModulePackageV2(pkg)
		if len(issues) > 0 {
			writeJSON(w, 400, map[string]any{"valid": false, "compatible": false, "issues": issues, "compatibilityIssues": []moduleValidationIssue{}})
			return
		}
		compatibility := []moduleValidationIssue{{"components", "runtime_unavailable"}}
		for i, c := range pkg.Components {
			for j := range c.Requires {
				compatibility = append(compatibility, moduleValidationIssue{fmt.Sprintf("components.%d.requires.%d", i, j), "capability_unavailable"})
			}
		}
		writeJSON(w, 200, map[string]any{"valid": true, "compatible": false, "issues": []moduleValidationIssue{}, "compatibilityIssues": compatibility})
	})
}
