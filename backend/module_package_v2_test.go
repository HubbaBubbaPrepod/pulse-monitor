package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func v2PackageJSON(t *testing.T, locations ...string) []byte {
	t.Helper()
	legacy := collectorPackage(t)
	components := []map[string]any{}
	for i, location := range locations {
		components = append(components, map[string]any{"id": string(rune('a' + i)), "location": location, "protocol": 2, "requires": []string{"state.read"}, "fields": []any{}, "triggers": []any{map[string]any{"type": "manual", "action": "inspect"}}, "binaries": legacy.Binaries})
	}
	raw, err := json.Marshal(map[string]any{"schemaVersion": 2, "protocol": 2, "id": "example", "version": "2.0.0", "name": "Example", "author": "Author", "description": "Example extension", "permissions": []string{}, "components": components})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestModulePackageV2AcceptsExecutionLocations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		locations []string
	}{{"monitor", []string{"monitor"}}, {"agent", []string{"agent"}}, {"combined", []string{"agent", "monitor"}}} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := decodeModulePackageV2(v2PackageJSON(t, tc.locations...))
			if err != nil {
				t.Fatal(err)
			}
			if issues := validateModulePackageV2(p); len(issues) > 0 {
				t.Fatal(issues)
			}
		})
	}
}

func TestModulePackageV2RejectsAmbiguousJSON(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{{"duplicate", `{"id":"first","id":"second"}`}, {"nested duplicate", `{"components":[{"id":"first","id":"second"}]}`}, {"unknown", `{"shell":"command"}`}, {"trailing", `{} {}`}, {"null", `null`}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeModulePackageV2([]byte(tc.raw)); err == nil {
				t.Fatal("accepted ambiguous JSON")
			}
		})
	}
}

func TestModulePackageV2RejectsNonCanonicalFields(t *testing.T) {
	raw := strings.Replace(string(v2PackageJSON(t, "monitor")), `"schemaVersion":2`, `"SchemaVersion":2`, 1)
	if _, err := decodeModulePackageV2([]byte(raw)); err == nil {
		t.Fatal("accepted non-canonical field name")
	}
}

func TestModulePackageV2RejectsJSONTypeCoercions(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"binary array", `{"components":[{"binaries":{"linux-amd64":{"content":[1,2]}}}]}`},
		{"null trigger field", `{"components":[{"triggers":[{"type":"manual","action":"inspect","seconds":null}]}]}`},
		{"null optional string", `{"author":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeModulePackageV2([]byte(tc.raw)); err == nil {
				t.Fatal("accepted a JSON type outside the schema")
			}
		})
	}
}

func TestModulePackageV2RejectsInvalidContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"path", func(p map[string]any) { p["id"] = "../escape" }},
		{"empty", func(p map[string]any) { p["components"] = []any{} }},
		{"duplicate component", func(p map[string]any) { c := p["components"].([]any); p["components"] = append(c, c[0]) }},
		{"location", func(p map[string]any) { p["components"].([]any)[0].(map[string]any)["location"] = "browser" }},
		{"agent event", func(p map[string]any) {
			p["components"].([]any)[0].(map[string]any)["triggers"] = []any{map[string]any{"type": "event", "events": []string{"server.offline"}}}
		}},
		{"wrong trigger fields", func(p map[string]any) {
			p["components"].([]any)[0].(map[string]any)["triggers"] = []any{map[string]any{"type": "manual", "action": "inspect", "seconds": 0}}
		}},
		{"interval bounds", func(p map[string]any) {
			p["components"].([]any)[0].(map[string]any)["triggers"] = []any{map[string]any{"type": "interval", "seconds": 4}}
		}},
		{"no trigger", func(p map[string]any) { p["components"].([]any)[0].(map[string]any)["triggers"] = []any{} }},
		{"capability wildcard", func(p map[string]any) { p["components"].([]any)[0].(map[string]any)["requires"] = []string{"state.*"} }},
		{"duplicate capability", func(p map[string]any) {
			p["components"].([]any)[0].(map[string]any)["requires"] = []string{"state.read", "state.read"}
		}},
		{"secret default", func(p map[string]any) {
			p["components"].([]any)[0].(map[string]any)["fields"] = []any{map[string]any{"key": "token", "type": "password", "label": "Token", "default": "not-public"}}
		}},
		{"digest", func(p map[string]any) {
			for _, v := range p["components"].([]any)[0].(map[string]any)["binaries"].(map[string]any) {
				v.(map[string]any)["sha256"] = strings.Repeat("0", 64)
			}
		}},
		{"wrong platform", func(p map[string]any) {
			b := p["components"].([]any)[0].(map[string]any)["binaries"].(map[string]any)
			for key, v := range b {
				delete(b, key)
				b["linux-arm64"] = v
				break
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(v2PackageJSON(t, "agent"), &value); err != nil {
				t.Fatal(err)
			}
			tc.edit(value)
			raw, _ := json.Marshal(value)
			p, err := decodeModulePackageV2(raw)
			if err == nil && len(validateModulePackageV2(p)) == 0 {
				t.Fatal("accepted invalid contract")
			}
		})
	}
}

func TestModulePackageV2RejectsAggregateBinarySize(t *testing.T) {
	p, err := decodeModulePackageV2(v2PackageJSON(t, "monitor"))
	if err != nil {
		t.Fatal(err)
	}
	var binary moduleBinary
	var target string
	for key, b := range p.Components[0].Binaries {
		target = key
		binary = b
		break
	}
	// Four separately valid artifacts must still respect the whole-package cap.
	content := make([]byte, 13*1024*1024)
	copy(content, binary.Content)
	digest := sha256.Sum256(content)
	binary.Content = content
	binary.SHA256 = hex.EncodeToString(digest[:])
	c := p.Components[0]
	c.Binaries = map[string]moduleBinary{target: binary}
	p.Components = nil
	for _, id := range []string{"a", "b", "c", "d"} {
		next := c
		next.ID = id
		p.Components = append(p.Components, next)
	}
	issues := validateModulePackageV2(p)
	for _, issue := range issues {
		if issue.Code == "package_too_large" {
			return
		}
	}
	t.Fatal("aggregate binary limit not enforced", issues)
}
