package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	gdisk "github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	gprocess "github.com/shirou/gopsutil/v4/process"
)

const gib = 1024 * 1024 * 1024
const mib = 1024 * 1024

type procTime struct {
	seconds float64
	created int64
}
type collector struct {
	metadata          map[int32]process
	last              time.Time
	previousIO        map[string]gdisk.IOCountersStat
	previousNet       gnet.IOCountersStat
	previousProcesses map[int32]procTime
	detailAt          time.Time
	details           agentReport
	logCursor         time.Time
	gpuAt             time.Time
	gpuRows           []gpu
	gpuMemory         map[int]*float64
	gpuLoad           map[int]*float64
	gpuStatus         string
}

func newCollector() *collector {
	return &collector{metadata: map[int32]process{}, previousIO: map[string]gdisk.IOCountersStat{}, previousProcesses: map[int32]procTime{}, logCursor: time.Now().UTC()}
}
func clamp(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Max(0, math.Min(100, v))
}
func delta(a, b uint64, seconds float64) float64 {
	if a < b || seconds <= 0 {
		return 0
	}
	return float64(a-b) / seconds
}
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}
func (c *collector) collect(ctx context.Context) (agentReport, error) {
	now := time.Now().UTC()
	seconds := now.Sub(c.last).Seconds()
	first := c.last.IsZero()
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return agentReport{}, fmt.Errorf("host info: %w", err)
	}
	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return agentReport{}, fmt.Errorf("memory: %w", err)
	}
	cores, _ := cpu.CountsWithContext(ctx, true)
	cpuInfo, _ := cpu.InfoWithContext(ctx)
	model := ""
	if len(cpuInfo) > 0 {
		model = cpuInfo[0].ModelName
	}
	r := agentReport{OS: truncate(strings.TrimSpace(info.Platform+" "+info.PlatformVersion), 80), Hardware: hardware{CPUCores: cores, CPUModel: model, MemoryGB: float64(memory.Total) / gib, Uptime: info.Uptime, Hostname: info.Hostname}, Sample: sample{Memory: clamp(memory.UsedPercent)}, Capabilities: map[string]string{}, Processes: []process{}, Disks: []disk{}, GPUs: []gpu{}, Connections: []connection{}, Packets: []packet{}, Logs: []logEntry{}}
	if r.OS == "" {
		r.OS = runtime.GOOS
	}
	usage, err := cpu.PercentWithContext(ctx, 0, false)
	if err == nil && len(usage) > 0 && !first {
		r.Sample.CPU = clamp(usage[0])
	}
	r.Capabilities["cpu"] = "available"
	if first {
		r.Capabilities["cpu"] = "warming up"
	}
	counters, err := gnet.IOCountersWithContext(ctx, false)
	if err == nil && len(counters) > 0 {
		if !first {
			r.Sample.Incoming = delta(counters[0].BytesRecv, c.previousNet.BytesRecv, seconds) / mib
			r.Sample.Outgoing = delta(counters[0].BytesSent, c.previousNet.BytesSent, seconds) / mib
		}
		c.previousNet = counters[0]
		r.Capabilities["network"] = "available"
	} else {
		r.Capabilities["network"] = "unavailable"
	}
	if c.detailAt.IsZero() || now.Sub(c.detailAt) >= 2*time.Second {
		detailSeconds := now.Sub(c.detailAt).Seconds()
		if c.detailAt.IsZero() {
			detailSeconds = 0
		}
		procs, err := gprocess.ProcessesWithContext(ctx)
		if err == nil {
			next := map[int32]procTime{}
			nextMetadata := map[int32]process{}
			for _, p := range procs {
				if ctx.Err() != nil {
					break
				}
				name, e := p.NameWithContext(ctx)
				if e != nil {
					continue
				}
				times, e := p.TimesWithContext(ctx)
				if e != nil {
					continue
				}
				memory, e := p.MemoryInfoWithContext(ctx)
				if e != nil {
					continue
				}
				created, _ := p.CreateTimeWithContext(ctx)
				total := times.User + times.System
				before, exists := c.previousProcesses[p.Pid]
				load := 0.0
				if exists && created == before.created && detailSeconds > 0 {
					load = clamp((total - before.seconds) / detailSeconds / float64(max(1, cores)) * 100)
				}
				next[p.Pid] = procTime{total, created}
				meta, known := c.metadata[p.Pid]
				if !known || !exists || created != before.created || meta.Name != name {
					command, _ := p.CmdlineWithContext(ctx)
					user, _ := p.UsernameWithContext(ctx)
					meta = processRow(p.Pid, name, user, command, "", 0, 0)
				}
				nextMetadata[p.Pid] = meta
				status, _ := p.StatusWithContext(ctx)
				meta.CPU = load
				meta.Memory = float64(memory.RSS) / mib
				meta.Status = strings.Join(status, ",")
				r.Processes = append(r.Processes, meta)
			}
			c.previousProcesses = next
			c.metadata = nextMetadata
			slices.SortFunc(r.Processes, func(a, b process) int {
				if a.Memory > b.Memory {
					return -1
				}
				if a.Memory < b.Memory {
					return 1
				}
				return a.PID - b.PID
			})
			if len(r.Processes) > 2048 {
				r.Processes = r.Processes[:2048]
			}
			r.Capabilities["processes"] = "available"
		} else {
			r.Capabilities["processes"] = "unavailable"
		}
		ioCounters, ioErr := gdisk.IOCountersWithContext(ctx)
		if ioErr != nil {
			r.Capabilities["diskIO"] = "unavailable"
		} else {
			r.Capabilities["diskIO"] = "available"
		}
		for name, v := range ioCounters {
			if runtime.GOOS == "linux" {
				if _, err := os.Stat(filepath.Join("/sys/class/block", name, "device")); err != nil {
					continue
				}
			}
			before := c.previousIO[name]
			if detailSeconds > 0 {
				r.Sample.Read += delta(v.ReadBytes, before.ReadBytes, detailSeconds) / mib
				r.Sample.Write += delta(v.WriteBytes, before.WriteBytes, detailSeconds) / mib
			}
		}
		partitions, err := gdisk.PartitionsWithContext(ctx, false)
		if err == nil {
			for _, p := range partitions {
				if len(r.Disks) >= 128 {
					break
				}
				u, e := gdisk.UsageWithContext(ctx, p.Mountpoint)
				if e != nil || u.Total == 0 {
					continue
				}
				key := filepath.Base(p.Device)
				v := ioCounters[key]
				if v.Name == "" {
					v = ioCounters[p.Device]
				}
				if v.Name == "" {
					v = ioCounters[strings.TrimRight(p.Mountpoint, `\/`)]
				}
				before := c.previousIO[v.Name]
				d := disk{ID: p.Mountpoint, Name: p.Device, Mount: p.Mountpoint, Filesystem: p.Fstype, TotalGB: float64(u.Total) / gib, UsedGB: float64(u.Used) / gib, Supported: map[string]bool{"read": v.Name != "", "write": v.Name != "", "iops": v.Name != "", "busy": v.Name != "" && runtime.GOOS == "linux", "latency": v.Name != "" && runtime.GOOS == "linux", "temperature": false}}
				if detailSeconds > 0 && v.Name != "" {
					d.Read = delta(v.ReadBytes, before.ReadBytes, detailSeconds) / mib
					d.Write = delta(v.WriteBytes, before.WriteBytes, detailSeconds) / mib
					ops := delta(v.ReadCount, before.ReadCount, detailSeconds) + delta(v.WriteCount, before.WriteCount, detailSeconds)
					d.IOPS = int(ops)
					d.Busy = clamp(delta(v.IoTime, before.IoTime, detailSeconds) / 10)
					if ops > 0 {
						d.Latency = (delta(v.ReadTime, before.ReadTime, detailSeconds) + delta(v.WriteTime, before.WriteTime, detailSeconds)) / ops
					}
				}
				r.Disks = append(r.Disks, d)
			}
		}
		c.previousIO = ioCounters
		names := map[int]string{}
		for _, p := range r.Processes {
			names[p.PID] = p.Name
		}
		connections, e := gnet.ConnectionsWithContext(ctx, "all")
		if e == nil {
			for i, v := range connections {
				if i >= 4096 {
					break
				}
				proto := "UDP"
				if v.Type == 1 {
					proto = "TCP"
				}
				remote := "—"
				if v.Raddr.IP != "" {
					remote = net.JoinHostPort(v.Raddr.IP, strconv.Itoa(int(v.Raddr.Port)))
				}
				r.Connections = append(r.Connections, connection{ID: fmt.Sprintf("%d-%s-%d-%s-%d", v.Pid, v.Laddr.IP, v.Laddr.Port, v.Raddr.IP, v.Raddr.Port), Local: net.JoinHostPort(v.Laddr.IP, strconv.Itoa(int(v.Laddr.Port))), Remote: remote, Protocol: proto, State: v.Status, Process: names[int(v.Pid)], PID: int(v.Pid)})
			}
			r.Capabilities["connections"] = "available"
		} else {
			r.Capabilities["connections"] = "unavailable: permissions or OS support"
		}
		c.detailAt = now
		c.details = r
	} else {
		r.Processes = slices.Clone(c.details.Processes)
		r.Disks = c.details.Disks
		r.Connections = c.details.Connections
		r.Sample.Read = c.details.Sample.Read
		r.Sample.Write = c.details.Sample.Write
		for _, key := range []string{"processes", "diskIO", "connections"} {
			r.Capabilities[key] = c.details.Capabilities[key]
		}
	}
	if now.Sub(c.gpuAt) >= 5*time.Second {
		prior := c.gpuRows
		c.gpuRows, c.gpuMemory, c.gpuLoad, c.gpuStatus = collectGPU(ctx)
		for i := range c.gpuRows {
			if c.gpuRows[i].Supported["utilization"] {
				for _, old := range prior {
					if old.ID == c.gpuRows[i].ID {
						c.gpuRows[i].History = append(slices.Clone(old.History), c.gpuRows[i].Utilization)
						if len(c.gpuRows[i].History) > 61 {
							c.gpuRows[i].History = c.gpuRows[i].History[len(c.gpuRows[i].History)-61:]
						}
						break
					}
				}
			}
		}
		c.gpuAt = now
	}
	r.GPUs = c.gpuRows
	r.Capabilities["gpu"] = c.gpuStatus
	for i := range r.Processes {
		r.Processes[i].GPU = c.gpuLoad[r.Processes[i].PID]
		r.Processes[i].GPUMemory = c.gpuMemory[r.Processes[i].PID]
	}
	if now.Sub(c.logCursor) >= 5*time.Second {
		entries, status := systemLogs(ctx, c.logCursor, now)
		r.Logs = entries
		r.Capabilities["systemLogs"] = status
		if status == "available" {
			if len(entries) >= 100 {
				for _, entry := range entries {
					stamp, err := time.Parse(time.RFC3339Nano, entry.Time)
					if err == nil && stamp.After(c.logCursor) {
						c.logCursor = stamp
					}
				}
			} else {
				c.logCursor = now
			}
		}
	} else {
		r.Capabilities["systemLogs"] = "collecting"
	}
	c.last = now
	return r, nil
}
func processRow(pid int32, name, user, command, status string, load, memory float64) process {
	return process{PID: int(pid), Name: truncate(name, 256), User: truncate(user, 256), Command: truncate(command, 4096), Status: status, CPU: load, Memory: memory}
}
