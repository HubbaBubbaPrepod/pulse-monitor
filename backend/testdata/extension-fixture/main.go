// This executable is an isolated UI test fixture, never a production module.
package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

func main() {
	var input struct {
		Protocol int               `json:"protocol"`
		Trigger  string            `json:"trigger"`
		Action   string            `json:"action"`
		Input    map[string]string `json:"input"`
	}
	if json.NewDecoder(os.Stdin).Decode(&input) != nil || input.Protocol != 2 {
		os.Exit(2)
	}
	id := input.Input["containerId"]
	data := map[string]any{"containerId": id, "action": input.Action, "fixture": true}
	if input.Trigger == "manual" {
		if input.Action == "logs" {
			data["logs"] = []string{"ISOLATED QA FIXTURE — no real Docker access", "selected target: " + id, "  indentation preserved; <script> stays plain text"}
		}
	} else {
		data = map[string]any{"engine": map[string]any{"version": "QA FIXTURE", "containers": 2, "running": 1, "truncated": false}, "containers": []map[string]any{{"id": strings.Repeat("a", 64), "name": "fixture-running", "image": "fixture:local", "state": "running", "cpu": 180, "memory": 512}, {"id": strings.Repeat("b", 64), "name": "fixture-stopped", "image": "fixture:local", "state": "exited", "cpu": 0, "memory": 0}}, "usage": map[string]any{"cpu": 160 + time.Now().Second()%30, "memory": 512}}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": 2, "status": "ok", "data": data})
}
