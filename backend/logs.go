package main

import "strings"

func logCategory(entry logEntry) string {
	source := strings.ToLower(entry.Source)
	message := strings.ToLower(entry.Message)
	if source == "pulse-agent" {
		return "agent"
	}
	if source == "ssh.service" || source == "sshd.service" || source == "sshd" || strings.Contains(source, "security-auditing") || strings.Contains(message, "pam_unix(sshd:auth)") {
		return "security"
	}
	// Only known informational chatter is telemetry; warnings and errors stay visible.
	if entry.Level == "INFO" && ((strings.HasPrefix(source, "netwtw") && strings.Contains(message, "connection telemetry fields")) || (source == "outlineservice" && strings.Contains(message, "network changed but outline is not connected"))) {
		return "telemetry"
	}
	return "system"
}
