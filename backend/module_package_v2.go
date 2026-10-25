package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

type moduleValidationIssue struct {
	Path string `json:"path"`
	Code string `json:"code"`
}
type modulePackageV2 struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Protocol      int                 `json:"protocol"`
	ID            string              `json:"id"`
	Version       string              `json:"version"`
	Name          string              `json:"name"`
	Author        string              `json:"author"`
	Description   string              `json:"description"`
	Permissions   []string            `json:"permissions"`
	Components    []moduleComponentV2 `json:"components"`
}
type moduleComponentV2 struct {
	Runtime              string                  `json:"runtime,omitempty"`
	OptionalCapabilities []string                `json:"optionalCapabilities,omitempty"`
	ID                   string                  `json:"id"`
	Location             string                  `json:"location"`
	Protocol             int                     `json:"protocol"`
	Requires             []string                `json:"requires"`
	Fields               []moduleField           `json:"fields"`
	Triggers             []moduleTriggerV2       `json:"triggers"`
	Binaries             map[string]moduleBinary `json:"binaries"`
	Views                []moduleViewV2          `json:"views,omitempty"`
}
type moduleViewV2 struct {
	moduleView
	RowKey  string   `json:"rowKey,omitempty"`
	Actions []string `json:"actions,omitempty"`
}
type moduleRowCondition struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}
type moduleTriggerV2 struct {
	Requires    []string            `json:"requires,omitempty"`
	When        *moduleRowCondition `json:"when,omitempty"`
	Type        string              `json:"type"`
	Seconds     *int                `json:"seconds,omitempty"`
	Events      *[]string           `json:"events,omitempty"`
	Action      *string             `json:"action,omitempty"`
	Label       string              `json:"label,omitempty"`
	Mutating    bool                `json:"mutating,omitempty"`
	InputFields []moduleField       `json:"inputFields,omitempty"`
	InputKey    string              `json:"inputKey,omitempty"`
	ResultView  string              `json:"resultView,omitempty"`
}

// Reject duplicate members before typed decoding: encoding/json otherwise keeps
// the last value and accepts case-insensitive field names.
func uniqueJSONValue(d *json.Decoder, depth int, member ...string) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeds limit")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	if len(member) > 0 {
		if token == nil && member[0] != "" && member[0] != "default" {
			return errors.New("Null member is not allowed")
		}
		if member[0] == "content" {
			if _, ok := token.(string); !ok {
				return errors.New("Binary content must be base64 text")
			}
		}
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("Invalid JSON delimiter")
	}
	seen := map[string]bool{}
	for d.More() {
		name := ""
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			var ok bool
			name, ok = key.(string)
			allowed := strings.Fields("schemaVersion protocol id version name author description permissions components location runtime optionalCapabilities requires fields triggers binaries key label type required default seconds events action sha256 content linux-amd64 linux-arm64 windows-amd64 wasip1-wasm views rowKey actions kind title columns unit mutating inputFields inputKey resultView when values")
			if !ok || seen[name] || !slices.Contains(allowed, name) {
				return errors.New("Duplicate JSON member")
			}
			seen[name] = true
		}
		if err := uniqueJSONValue(d, depth+1, name); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
func decodeModulePackageV2(raw []byte) (modulePackageV2, error) {
	var p modulePackageV2
	if len(raw) > 68*1024*1024 || len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return p, errors.New("Invalid package")
	}
	scan := json.NewDecoder(bytes.NewReader(raw))
	if err := uniqueJSONValue(scan, 0); err != nil {
		return p, errors.New("Ambiguous package JSON")
	}
	if _, err := scan.Token(); !errors.Is(err, io.EOF) {
		return p, errors.New("Expected one package")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, errors.New("Invalid package fields")
	}
	return p, nil
}

var moduleCapabilityName = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)*$`)

func validModuleNames(names []string, limit int) bool {
	if len(names) > limit {
		return false
	}
	seen := map[string]bool{}
	for _, name := range names {
		if len(name) > 128 || !moduleCapabilityName.MatchString(name) || seen[name] {
			return false
		}
		seen[name] = true
	}
	return true
}
func validateModulePackageV2(p modulePackageV2) []moduleValidationIssue {
	return validateModulePackageV2Content(p, true)
}
func validateModulePackageV2Content(p modulePackageV2, content bool) []moduleValidationIssue {
	issues := []moduleValidationIssue{}
	add := func(path, code string) { issues = append(issues, moduleValidationIssue{path, code}) }
	if p.SchemaVersion != 2 || p.Protocol != 2 || !validID.MatchString(p.ID) || !moduleVersion.MatchString(p.Version) || len(p.Version) > 48 || len(p.Name) == 0 || len(p.Name) > 64 || len(p.Author) > 128 || len(p.Description) > 2048 || len(p.Permissions) > 16 {
		add("package", "invalid_metadata")
	}
	for _, permission := range p.Permissions {
		if len(permission) > 512 {
			add("permissions", "invalid_permission")
			break
		}
	}
	if len(p.Components) < 1 || len(p.Components) > 16 {
		add("components", "invalid_component_count")
		return issues
	}
	seen := map[string]bool{}
	total := 0
	for i, c := range p.Components {
		path := fmt.Sprintf("components.%d", i)
		if !validID.MatchString(c.ID) || seen[strings.ToLower(c.ID)] {
			add(path+".id", "invalid_component_id")
		}
		seen[strings.ToLower(c.ID)] = true
		if !slices.Contains([]string{"agent", "monitor"}, c.Location) || c.Protocol != 2 || !slices.Contains([]string{"", "native", "wasip1"}, c.Runtime) {
			add(path, "invalid_runtime")
		}
		if !validModuleNames(c.Requires, 32) {
			add(path+".requires", "invalid_capability")
		}
		// Apply the existing platform, binary digest, field and metadata constraints.
		legacy := moduleManifest{SchemaVersion: 1, Protocol: 1, ID: p.ID, Version: p.Version, Name: p.Name, Author: p.Author, Description: p.Description, Permissions: p.Permissions, Fields: c.Fields, Binaries: c.Binaries}
		for _, v := range c.Views {
			legacy.Views = append(legacy.Views, v.moduleView)
		}
		var artifactErr error
		if c.Runtime == "wasip1" {
			artifactErr = validateModuleMetadata(legacy)
			if artifactErr == nil {
				artifactErr = validateWASIArtifacts(c.Binaries, content)
			}
		} else {
			artifactErr = validateModule(legacy, content)
		}
		if artifactErr != nil {
			add(path, "invalid_component_artifacts_or_fields")
		}
		if !validWASICapabilities(c) {
			add(path, "invalid_runtime_capabilities")
		}
		for _, b := range c.Binaries {
			total += len(b.Content)
		}
		for _, f := range c.Fields {
			if c.Runtime == "wasip1" && !validWASIHostField(c.Location, f) {
				add(path+".fields", "invalid_reserved_host_field")
			}
			if f.Default == nil {
				continue
			}
			switch f.Type {
			case "password":
				if f.Default != "" {
					add(path+".fields", "secret_default_forbidden")
				}
			case "text":
				v, ok := f.Default.(string)
				if !ok || len(v) > 4096 {
					add(path+".fields", "invalid_default")
				}
			case "number":
				if _, ok := f.Default.(float64); !ok {
					add(path+".fields", "invalid_default")
				}
			case "boolean":
				if _, ok := f.Default.(bool); !ok {
					add(path+".fields", "invalid_default")
				}
			}
		}
		if len(c.Triggers) < 1 || len(c.Triggers) > 16 {
			add(path+".triggers", "invalid_trigger_count")
		}
		for j, t := range c.Triggers {
			valid := false
			if len(t.Requires) > 0 && (c.Runtime != "wasip1" || t.Type != "manual" || !validModuleNames(t.Requires, 32)) {
				add(path+".triggers", "invalid_action_capabilities")
			}
			for _, required := range t.Requires {
				if !slices.Contains(c.Requires, required) && !slices.Contains(c.OptionalCapabilities, required) {
					add(path+".triggers", "undeclared_action_capability")
				}
			}
			if c.Runtime == "wasip1" && t.Mutating && !slices.Contains(c.Requires, "actions.execute") && !slices.Contains(c.OptionalCapabilities, "actions.execute") {
				add(path+".triggers", "undeclared_mutation_capability")
			}
			switch t.Type {
			case "interval":
				valid = t.Seconds != nil && *t.Seconds >= 5 && *t.Seconds <= 86400 && t.Action == nil && t.Events == nil
			case "event":
				valid = c.Location == "monitor" && t.Events != nil && len(*t.Events) > 0 && validModuleNames(*t.Events, 32) && t.Action == nil && t.Seconds == nil
			case "manual":
				valid = t.Action != nil && validID.MatchString(*t.Action) && t.Events == nil && t.Seconds == nil
			}
			if t.Type != "manual" && (t.Mutating || len(t.InputFields) > 0 || t.InputKey != "" || t.ResultView != "" || t.When != nil) {
				valid = false
			}
			if len(t.Label) > 128 || !slices.Contains([]string{"", "logs", "cards", "table"}, t.ResultView) || len(t.InputFields) > 32 {
				valid = false
			}
			if t.When != nil {
				if !validID.MatchString(t.When.Key) || len(t.When.Values) == 0 || len(t.When.Values) > 16 {
					valid = false
				}
				for _, value := range t.When.Values {
					if len(value) == 0 || len(value) > 128 {
						valid = false
					}
				}
			}
			inputKeys := map[string]bool{}
			for _, f := range t.InputFields {
				if !validID.MatchString(f.Key) || inputKeys[f.Key] || len(f.Label) > 128 || !slices.Contains([]string{"text", "password", "number", "boolean"}, f.Type) || f.Default != nil {
					valid = false
				}
				inputKeys[f.Key] = true
			}
			if t.InputKey != "" && !inputKeys[t.InputKey] {
				valid = false
			}
			if !valid {
				add(fmt.Sprintf("%s.triggers.%d", path, j), "invalid_trigger")
			}
		}
		actions := map[string]bool{}
		for _, t := range c.Triggers {
			if t.Action != nil {
				if actions[*t.Action] {
					add(path+".triggers", "duplicate_action")
				}
				actions[*t.Action] = true
			}
		}
		for _, v := range c.Views {
			if v.RowKey != "" && !validID.MatchString(v.RowKey) {
				add(path+".views", "invalid_row_key")
			}
			if len(v.Actions) > 16 {
				add(path+".views", "invalid_view_actions")
			}
			for _, action := range v.Actions {
				if !actions[action] {
					add(path+".views", "unknown_action")
				}
			}
		}
	}
	// The persisted manifest uses the same bounded reader as other private metadata.
	metadata := p
	metadata.Components = append([]moduleComponentV2(nil), p.Components...)
	for i, c := range metadata.Components {
		binaries := map[string]moduleBinary{}
		for target, b := range c.Binaries {
			b.Content = nil
			binaries[target] = b
		}
		metadata.Components[i].Binaries = binaries
	}
	encoded, err := json.Marshal(metadata)
	if err != nil || len(encoded) > 2*1024*1024 {
		add("package", "metadata_too_large")
	}
	if total > 48*1024*1024 {
		add("components", "package_too_large")
	}
	return issues
}
