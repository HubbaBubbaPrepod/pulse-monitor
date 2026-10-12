package main

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

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

func demoServers() []server {
	return []server{
		{ID: "production-01", Name: "production-01", Address: "https://192.168.1.10:9443", OS: "Ubuntu 24.04 LTS", Group: "Production", Online: true, CPU: 24.8, Memory: 62.4, Disk: 38.2, Interval: 5, Threshold: 85},
		{ID: "windows-02", Name: "windows-02", Address: "https://192.168.1.20:9443", OS: "Windows Server 2022", Group: "Production", Online: true, CPU: 18.6, Memory: 44.2, Disk: 56.7, Interval: 5, Threshold: 85},
		{ID: "backup-03", Name: "backup-03", Address: "https://192.168.1.30:9443", OS: "Debian 12", Group: "Backup", Online: false, CPU: 0, Memory: 0, Disk: 71.2, Interval: 10, Threshold: 90},
	}
}
func (a *app) snapshot(s server, period int) snapshot {
	if value, ok := a.agentSnapshot(s, period); ok {
		return value
	}
	now := time.Now().UTC()
	tick := int(time.Since(a.started) / (500 * time.Millisecond))
	result := snapshot{ServerID: s.ID, Source: "demo", Tick: tick, At: now.Format(time.RFC3339Nano), Samples: []sample{}, Processes: []process{}, Disks: []disk{}, GPUs: []gpu{}, Connections: []connection{}, Packets: []packet{}}
	for i := range 61 {
		value := sample{}
		if s.Online {
			x := float64(i*period)/60 + float64(tick)*.1
			wave := math.Sin(x*.46)*6 + math.Sin(x*1.8)*3
			spike := math.Pow(max(0, math.Sin(x*.39)), 18) * 23
			value = sample{CPU: max(3, min(96, s.CPU+wave+spike)), Memory: s.Memory + math.Sin(x*.16)*2, Incoming: 48 + wave*1.7 + spike, Outgoing: 25 + math.Sin(x*.2)*9, Read: 16 + math.Abs(wave)*3, Write: 8 + math.Abs(math.Sin(x*.8))*16}
		}
		result.Samples = append(result.Samples, value)
	}
	if !s.Online {
		return result
	}
	windows := strings.HasPrefix(s.OS, "Windows")
	names := []string{"nginx", "postgres", "node", "systemd", "redis-server", "docker", "sshd", "pulse-agent"}
	users := []string{"www-data", "postgres", "node", "root", "redis", "root", "root", "pulse"}
	commands := []string{"nginx: worker process", "postgres: checkpointer", "/usr/bin/node /srv/app/server.js", "/sbin/init", "redis-server 127.0.0.1:6379", "/usr/bin/dockerd", "/usr/sbin/sshd -D", "pulse-agent serve"}
	if windows {
		names = []string{"w3wp.exe", "sqlservr.exe", "node.exe", "svchost.exe", "MsMpEng.exe", "pulse-agent.exe"}
		users = []string{"IIS APPPOOL", "NT SERVICE", "Administrator", "SYSTEM", "SYSTEM", "SYSTEM"}
		commands = []string{`C:\Windows\System32\inetsrv\w3wp.exe`, `C:\Program Files\Microsoft SQL Server\sqlservr.exe`, "node.exe server.js", "svchost.exe -k netsvcs", "Windows Defender Service", "pulse-agent.exe serve"}
	}
	pids := []int{1142, 2876, 4210, 1, 1854, 2201, 931, 9012}
	cpus := []float64{12.3, 8.1, 4.7, 1.2, .8, .6, .1, .3}
	memory := []float64{512, 1229, 420, 96, 148, 240, 12, 28}
	for i, name := range names {
		result.Processes = append(result.Processes, process{PID: pids[i], Name: name, User: users[i], CPU: max(.1, cpus[i]+math.Sin(float64(tick)*.2+float64(i))*.3), Memory: memory[i], Command: commands[i], Status: "Работает"})
	}
	last := result.Samples[60]
	mounts := []string{"/", "/data"}
	filesystems := []string{"ext4", "xfs"}
	if windows {
		mounts = []string{"C:", "D:"}
		filesystems = []string{"NTFS", "NTFS"}
	}
	result.Disks = []disk{
		{ID: "disk-0", Name: "Samsung NVMe 980 PRO", Mount: mounts[0], Filesystem: filesystems[0], TotalGB: 500, UsedGB: s.Disk * 5, Read: last.Read, Write: last.Write, IOPS: 1240 + tick%120, Busy: 23 + math.Sin(float64(tick)*.1)*5, Latency: .8, Temperature: 41},
		{ID: "disk-1", Name: "Data SSD", Mount: mounts[1], Filesystem: filesystems[1], TotalGB: 1000, UsedGB: 276, Read: last.Read * .4, Write: last.Write * .3, IOPS: 430 + tick%60, Busy: 12, Latency: 1.2, Temperature: 36},
	}
	// Only this fixture advertises a GPU. Future agents supply actual hardware capabilities.
	if s.ID == "production-01" {
		values := []float64{}
		for i := range 30 {
			values = append(values, 35+math.Sin(float64(i+tick)*.14)*18)
		}
		result.GPUs = []gpu{{ID: "gpu-0", Name: "NVIDIA RTX 4090", Utilization: values[29], MemoryUsed: 8.4, MemoryTotal: 24, Temperature: 58 + math.Sin(float64(tick)*.1)*3, Power: 165, Fan: 42, History: values}}
		result.Processes = append(result.Processes, process{PID: 6701, Name: "python3", User: "inference", CPU: 14, Memory: 3200, Command: "/usr/bin/python3 /srv/inference.py", Status: "Работает"})
		for i := range result.Processes {
			share := 0.0
			switch result.Processes[i].Name {
			case "python3":
				share = .8
			case "node":
				share = .15
			case "docker":
				share = .05
			}
			util := result.GPUs[0].Utilization * share
			mem := 8.4 * 1024 * share
			result.Processes[i].GPU = &util
			result.Processes[i].GPUMemory = &mem
		}

	}
	u, _ := url.Parse(s.Address)
	host := u.Hostname()
	appName := names[0]
	result.Connections = []connection{
		{ID: "https", Local: host + ":443", Remote: "192.0.2.42:52184", Protocol: "TCP", State: "ESTABLISHED", Process: appName, PID: pids[0], Incoming: 28.4, Outgoing: 18.2},
		{ID: "db", Local: host + ":5432", Remote: "192.168.1.50:49820", Protocol: "TCP", State: "ESTABLISHED", Process: names[1], PID: pids[1], Incoming: 7.6, Outgoing: 3.2},
		{ID: "dns", Local: host + ":53012", Remote: "192.0.2.53:53", Protocol: "UDP", State: "ACTIVE", Process: names[2], PID: pids[2], Incoming: .2, Outgoing: .1},
		{ID: "ssh", Local: host + ":22", Remote: "198.51.100.18:61442", Protocol: "TCP", State: "ESTABLISHED", Process: "sshd", PID: 931, Incoming: .1, Outgoing: .3},
	}
	if windows {
		result.Connections[1].Local = host + ":1433"
		result.Connections[3].Local = host + ":3389"
		result.Connections[3].Process = "svchost.exe"
	}
	for i := range 24 {
		c := result.Connections[(i+tick)%len(result.Connections)]
		incoming := (i+tick)%3 != 0
		source, destination := c.Remote, c.Local
		direction := "in"
		if !incoming {
			source, destination = c.Local, c.Remote
			direction = "out"
		}
		protocol := c.Protocol
		flags, summary := "ACK, PSH", "TLS application data · encrypted payload"
		length := 1460
		if c.ID == "dns" {
			protocol = "DNS"
			flags = "—"
			summary = "DNS query/response · A example.test"
			length = 84
		} else if c.ID == "db" {
			summary = "Database connection · data segment"
			length = 512
		} else if c.ID == "ssh" {
			summary = "Encrypted remote session"
			length = 128
		}
		result.Packets = append(result.Packets, packet{ID: fmt.Sprintf("%s-%d-%d", s.ID, tick, i), Time: now.Add(-time.Duration(i) * 250 * time.Millisecond).Format(time.RFC3339Nano), Direction: direction, Source: source, Destination: destination, Protocol: protocol, Length: length, Flags: flags, Process: c.Process, Summary: summary})
	}
	return result
}
func demoLog(s server, index int) (string, string, string) {
	messages := [][3]string{{"INFO", "pulse", "Демо: монитор собирает данные"}, {"INFO", "nginx", "Демо: входящий HTTPS запрос обработан"}, {"WARN", "kernel", "Демо: кратковременный рост дисковых операций"}, {"INFO", "postgres", "Демо: контрольная точка сохранена"}, {"ERROR", "network", "Демо: таймаут соединения, повтор запланирован"}}
	row := messages[index%len(messages)]
	if strings.HasPrefix(s.OS, "Windows") && row[1] == "kernel" {
		row[1] = "Windows Event Log"
	}
	return row[0], row[1], row[2]
}
