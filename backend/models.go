package main

import "time"

type sample struct {
	At       string  `json:"at,omitempty"`
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	Incoming float64 `json:"incoming"`
	Outgoing float64 `json:"outgoing"`
	Read     float64 `json:"read"`
	Write    float64 `json:"write"`
}
type process struct {
	PID       int      `json:"pid"`
	Name      string   `json:"name"`
	User      string   `json:"user"`
	CPU       float64  `json:"cpu"`
	Memory    float64  `json:"memory"`
	Command   string   `json:"command"`
	Status    string   `json:"status"`
	GPU       *float64 `json:"gpu"`
	GPUMemory *float64 `json:"gpuMemory"`
}
type disk struct {
	Supported   map[string]bool `json:"supported,omitempty"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Mount       string          `json:"mount"`
	Filesystem  string          `json:"filesystem"`
	TotalGB     float64         `json:"totalGB"`
	UsedGB      float64         `json:"usedGB"`
	Read        float64         `json:"read"`
	Write       float64         `json:"write"`
	IOPS        int             `json:"iops"`
	Busy        float64         `json:"busy"`
	Latency     float64         `json:"latency"`
	Temperature float64         `json:"temperature"`
}
type gpu struct {
	Supported   map[string]bool `json:"supported,omitempty"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Utilization float64         `json:"utilization"`
	MemoryUsed  float64         `json:"memoryUsed"`
	MemoryTotal float64         `json:"memoryTotal"`
	Temperature float64         `json:"temperature"`
	Power       float64         `json:"power"`
	Fan         float64         `json:"fan"`
	History     []float64       `json:"history"`
}
type connection struct {
	ID       string  `json:"id"`
	Local    string  `json:"local"`
	Remote   string  `json:"remote"`
	Protocol string  `json:"protocol"`
	State    string  `json:"state"`
	Process  string  `json:"process"`
	PID      int     `json:"pid"`
	Incoming float64 `json:"incoming"`
	Outgoing float64 `json:"outgoing"`
}
type packet struct {
	ID          string `json:"id"`
	Time        string `json:"time"`
	Direction   string `json:"direction"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	Length      int    `json:"length"`
	Flags       string `json:"flags"`
	Process     string `json:"process"`
	Summary     string `json:"summary"`
}
type snapshot struct {
	Hardware     hardware          `json:"hardware,omitempty"`
	Capabilities map[string]string `json:"capabilities,omitempty"`
	ServerID     string            `json:"serverId"`
	Source       string            `json:"source"`
	Tick         int               `json:"tick"`
	At           string            `json:"at"`
	Samples      []sample          `json:"samples"`
	Processes    []process         `json:"processes"`
	Disks        []disk            `json:"disks"`
	GPUs         []gpu             `json:"gpus"`
	Connections  []connection      `json:"connections"`
	Packets      []packet          `json:"packets"`
}

func (a *app) snapshot(s server, period int) snapshot {
	if value, ok := a.agentSnapshot(s, period); ok {
		return value
	}
	return snapshot{ServerID: s.ID, Source: "waiting", At: time.Now().UTC().Format(time.RFC3339Nano), Samples: []sample{}, Processes: []process{}, Disks: []disk{}, GPUs: []gpu{}, Connections: []connection{}, Packets: []packet{}}
}
