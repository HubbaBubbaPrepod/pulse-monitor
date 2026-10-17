package main

import (
	"encoding/json"
	"testing"
)

func TestPreferencesPersistAndRejectCSS(t *testing.T) {
	dir := t.TempDir()
	a, err := openTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"interval":0,"motion":true,"compact":true,"locale":"en","themeId":"dark","themes":[]}`
	w := request(t, a, "PUT", "/api/preferences", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	a, err = openTestApp(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	w = request(t, a, "GET", "/api/preferences", "")
	var p map[string]any
	json.Unmarshal(w.Body.Bytes(), &p)
	if p["locale"] != "en" || p["compact"] != true {
		t.Fatal(p)
	}
	bad := `{"interval":0,"motion":true,"compact":true,"locale":"xx","themeId":"dark","themes":[]}`
	if w := request(t, a, "PUT", "/api/preferences", bad); w.Code != 400 {
		t.Fatal("invalid language accepted", w.Code)
	}
}

func TestPreferencesSupportLongUnicodeThemeNames(t *testing.T) {
	p := preferences{Interval: 5, Motion: true, Locale: "ru", ThemeID: "custom", Themes: []themeDefinition{{SchemaVersion: 1, ID: "custom", Name: "Моя прекрасная пользовательская тема для сервера", Mode: "dark", Radius: 14, Colors: map[string]string{}}}}
	for _, key := range themeColors {
		p.Themes[0].Colors[key] = "#112233"
	}
	if err := validatePreferences(p); err != nil {
		t.Fatal(err)
	}
}
