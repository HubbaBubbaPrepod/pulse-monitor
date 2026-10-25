//go:build integration

package main

import (
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var wasiPackagePath = flag.String("wasi-package", "", "Explicit WASI package fixture path")

func TestBuiltWASIPackageInstallation(t *testing.T) {
	if *wasiPackagePath == "" {
		t.Skip("Supply an explicit WASI package")
	}
	raw, err := os.ReadFile(*wasiPackagePath)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := decodeModulePackageV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	if issues := validateModulePackageV2(pkg); len(issues) > 0 {
		t.Fatal(issues)
	}
	a, err := newTestApp(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if w := request(t, a, "POST", "/api/extensions", string(raw)); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	installed, err := a.extension(pkg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Components[0].Runtime != "wasip1" {
		t.Fatal("runtime metadata lost")
	}
	w := request(t, a, "GET", "/api/extensions", "")
	var public []modulePackageV2
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &public) != nil || len(public) != 1 {
		t.Fatal("installed metadata unavailable")
	}
	for _, binary := range public[0].Components[0].Binaries {
		if len(binary.Content) > 0 {
			t.Fatal("artifact leaked into metadata")
		}
	}
}
