package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type themeOptions struct {
	Font        string  `json:"font"`
	ContentFont string  `json:"contentFont"`
	CodeFont    string  `json:"codeFont"`
	FontSize    int     `json:"fontSize"`
	CodeSize    int     `json:"codeSize"`
	Italic      bool    `json:"italic"`
	Transparent bool    `json:"transparent"`
	Contrast    float64 `json:"contrast"`
	Pointer     bool    `json:"pointer"`
}
type appearance struct {
	Mode         string `json:"mode"`
	Separate     bool   `json:"separate"`
	LightThemeID string `json:"lightThemeId"`
	DarkThemeID  string `json:"darkThemeId"`
	MotionMode   string `json:"motionMode"`
}
type themeDefinition struct {
	Accent        string            `json:"accent,omitempty"`
	Options       *themeOptions     `json:"options,omitempty"`
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Mode          string            `json:"mode"`
	Radius        float64           `json:"radius"`
	Colors        map[string]string `json:"colors"`
}
type preferences struct {
	Appearance *appearance       `json:"appearance,omitempty"`
	Interval   int               `json:"interval"`
	Motion     bool              `json:"motion"`
	Compact    bool              `json:"compact"`
	Locale     string            `json:"locale"`
	ThemeID    string            `json:"themeId"`
	Themes     []themeDefinition `json:"themes"`
}

var themeColors = []string{"bg", "surface", "surface-hover", "raised", "text", "muted", "subtle", "border", "track", "chart-primary", "chart-secondary", "chart-tertiary"}
var hexColor = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)

func validatePreferences(p preferences) error {
	if !slices.Contains([]int{0, 2, 5, 10, 30}, p.Interval) || !slices.Contains([]string{"ru", "en"}, p.Locale) || !validID.MatchString(p.ThemeID) || len(p.Themes) > 16 {
		return errors.New("Invalid preferences")
	}
	ids := map[string]bool{"dark": true, "light": true, "tokyo-night": true}
	for _, theme := range p.Themes {
		if theme.SchemaVersion != 1 || !validID.MatchString(theme.ID) || ids[theme.ID] || strings.TrimSpace(theme.Name) == "" || utf8.RuneCountInString(theme.Name) > 64 || !slices.Contains([]string{"dark", "light"}, theme.Mode) || theme.Radius < 0 || theme.Radius > 24 || len(theme.Colors) != len(themeColors) {
			return errors.New("Invalid theme")
		}
		if theme.Accent != "" && !hexColor.MatchString(theme.Accent) {
			return errors.New("Invalid accent")
		}
		if o := theme.Options; o != nil {
			if !slices.Contains([]string{"system", "arial", "segoe"}, o.Font) || !slices.Contains([]string{"inherit", "system", "arial", "segoe"}, o.ContentFont) || !slices.Contains([]string{"system", "consolas", "courier"}, o.CodeFont) || o.FontSize < 12 || o.FontSize > 20 || o.CodeSize < 10 || o.CodeSize > 20 || o.Contrast < 0 || o.Contrast > 100 {
				return errors.New("Invalid typography")
			}
		}
		for _, key := range themeColors {
			if !hexColor.MatchString(theme.Colors[key]) {
				return errors.New("Invalid theme color")
			}
		}
		ids[theme.ID] = true
	}
	if v := p.Appearance; v != nil {
		if !slices.Contains([]string{"system", "dark", "light"}, v.Mode) || !slices.Contains([]string{"system", "on", "off"}, v.MotionMode) || !ids[v.LightThemeID] || !ids[v.DarkThemeID] {
			return errors.New("Invalid appearance")
		}
	}
	if !ids[p.ThemeID] {
		return errors.New("Unknown theme")
	}
	return nil
}
func (a *app) preferenceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/preferences", func(w http.ResponseWriter, r *http.Request) {
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		p := preferences{Interval: 5, Motion: true, Locale: "ru", ThemeID: "dark", Themes: []themeDefinition{}}
		raw, err := os.ReadFile(filepath.Join(a.dir, "preferences.json"))
		if err == nil {
			err = json.Unmarshal(raw, &p)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			apiError(w, 500, "Could not read preferences")
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("PUT /api/preferences", func(w http.ResponseWriter, r *http.Request) {
		var p preferences
		if err := decodeJSON(w, r, &p); err != nil {
			apiError(w, 400, "Invalid preferences JSON")
			return
		}
		if err := validatePreferences(p); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		if err := saveJSON(filepath.Join(a.dir, "preferences.json"), p); err != nil {
			apiError(w, 500, "Could not save preferences")
			return
		}
		writeJSON(w, 200, p)
	})
}
