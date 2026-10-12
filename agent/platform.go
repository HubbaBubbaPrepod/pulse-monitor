package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func commandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		if name == "powershell.exe" {
			name = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
			for i := range args {
				if i > 0 && args[i-1] == "-Command" {
					args[i] = `[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); ` + args[i]
				}
			}
		}
		if name == "nvidia-smi" {
			if _, err := exec.LookPath(name); err != nil {
				for _, candidate := range []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "nvidia-smi.exe"), filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe")} {
					if _, err := os.Stat(candidate); err == nil {
						name = candidate
						break
					}
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	configureCommand(cmd)
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &limitedBuffer{}
	err := cmd.Run()
	return out.Bytes(), err
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2*1024*1024 {
		return 0, fmt.Errorf("command output exceeds limit")
	}
	return b.Buffer.Write(p)
}
func numeric(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }
func collectGPU(ctx context.Context) ([]gpu, map[int]*float64, map[int]*float64, string) {
	rows := []gpu{}
	memory := map[int]*float64{}
	load := map[int]*float64{}
	adapterEngines := map[int]map[int]float64{}
	raw, err := commandOutput(ctx, "nvidia-smi", "--query-gpu=uuid,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,fan.speed", "--format=csv,noheader,nounits")
	if err == nil {
		records, _ := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
		for _, r := range records {
			if len(r) != 8 {
				continue
			}
			rows = append(rows, gpu{ID: strings.TrimSpace(r[0]), Name: strings.TrimSpace(r[1]), Utilization: clamp(numeric(r[2])), MemoryUsed: numeric(r[3]) / 1024, MemoryTotal: numeric(r[4]) / 1024, Temperature: numeric(r[5]), Power: numeric(r[6]), Fan: numeric(r[7]), History: []float64{clamp(numeric(r[2]))}})
			rows[len(rows)-1].Supported = map[string]bool{}
			for i, key := range []string{"utilization", "memoryUsed", "memoryTotal", "temperature", "power", "fan"} {
				_, e := strconv.ParseFloat(strings.TrimSpace(r[i+2]), 64)
				rows[len(rows)-1].Supported[key] = e == nil
			}
		}
		raw, e := commandOutput(ctx, "nvidia-smi", "--query-compute-apps=pid,used_gpu_memory", "--format=csv,noheader,nounits")
		if e == nil {
			records, _ := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
			for _, r := range records {
				if len(r) != 2 {
					continue
				}
				pid, _ := strconv.Atoi(strings.TrimSpace(r[0]))
				if value, e := strconv.ParseFloat(strings.TrimSpace(r[1]), 64); e == nil {
					value += valueOrZero(memory[pid])
					memory[pid] = &value
				}
			}
		}
		if runtime.GOOS == "linux" {
			raw, e := commandOutput(ctx, "nvidia-smi", "pmon", "-c", "1", "-s", "u")
			if e == nil {
				for _, line := range strings.Split(string(raw), "\n") {
					cols := strings.Fields(line)
					if len(cols) < 4 || strings.HasPrefix(strings.TrimSpace(line), "#") {
						continue
					}
					pid, e := strconv.Atoi(cols[1])
					value, vErr := strconv.ParseFloat(cols[3], 64)
					if e == nil && vErr == nil {
						value = clamp(value + valueOrZero(load[pid]))
						load[pid] = &value
					}
				}
			}
		}
	}
	if runtime.GOOS == "windows" {
		// GPU Engine counters expose PID-specific utilization on WDDM drivers.
		script := `$ErrorActionPreference='Stop'; $rows=@((Get-Counter '\GPU Engine(*)\Utilization Percentage' -ErrorAction Stop).CounterSamples | ForEach-Object { if ($_.InstanceName -match 'pid_(\d+)_.*phys_(\d+)_eng_(\d+)_') { [pscustomobject]@{pid=[int]$Matches[1];physical=[int]$Matches[2];engine=[int]$Matches[3];load=[double]$_.CookedValue} } }); ConvertTo-Json -Compress -InputObject $rows`
		raw, e := commandOutput(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if e == nil {
			var counters []struct {
				PID      int     `json:"pid"`
				Load     float64 `json:"load"`
				Physical int     `json:"physical"`
				Engine   int     `json:"engine"`
			}
			if json.Unmarshal(raw, &counters) == nil {
				for _, v := range counters {
					if adapterEngines[v.Physical] == nil {
						adapterEngines[v.Physical] = map[int]float64{}
					}
					adapterEngines[v.Physical][v.Engine] += v.Load
					value := clamp(v.Load + valueOrZero(load[v.PID]))
					load[v.PID] = &value
				}
			}
		}
		if len(rows) == 0 {
			raw, e := commandOutput(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; ConvertTo-Json -Compress -InputObject @(Get-CimInstance Win32_VideoController | Select-Object Name,PNPDeviceID,AdapterRAM)`)
			if e == nil {
				var devices []struct {
					Name        string
					PNPDeviceID string
					AdapterRAM  uint64
				}
				if json.Unmarshal(raw, &devices) == nil {
					for _, d := range devices {
						if d.AdapterRAM == 0 {
							continue
						}
						rows = append(rows, gpu{ID: d.PNPDeviceID, Name: d.Name, Supported: map[string]bool{"utilization": false, "memoryUsed": false, "memoryTotal": false, "temperature": false, "power": false, "fan": false}, History: []float64{}})
					}
				}
				if len(rows) == 1 && len(adapterEngines) == 1 {
					for _, engines := range adapterEngines {
						util := 0.0
						for _, value := range engines {
							util = max(util, value)
						}
						rows[0].Utilization = clamp(util)
						rows[0].Supported["utilization"] = true
						rows[0].History = []float64{clamp(util)}
					}
				}
			}
		}
	}
	status := "unavailable: no supported GPU telemetry driver"
	if len(rows) > 0 {
		status = "available"
	}
	return rows, memory, load, status
}
func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// Read the oldest page, stopping the child after a bounded batch instead of
// discarding the oldest entries when catching up after a network outage.
func journalPage(ctx context.Context, since, until time.Time) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "journalctl", "--no-pager", "--output=json", "--since", since.Add(time.Microsecond).Format(time.RFC3339Nano), "--until", until.Format(time.RFC3339Nano))
	configureCommand(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = &limitedBuffer{}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	scan := bufio.NewScanner(stdout)
	scan.Buffer(make([]byte, 4096), 256*1024)
	var out limitedBuffer
	count := 0
	for scan.Scan() {
		if _, err := out.Write(append(bytes.Clone(scan.Bytes()), '\n')); err != nil {
			cancel()
			cmd.Wait()
			return nil, err
		}
		count++
		if count == 100 {
			cancel()
			break
		}
	}
	scanErr := scan.Err()
	waitErr := cmd.Wait()
	if count == 100 {
		return out.Bytes(), nil
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return out.Bytes(), waitErr
}
func systemLogs(ctx context.Context, since, until time.Time) ([]logEntry, string) {
	entries := []logEntry{}
	if runtime.GOOS == "linux" {
		raw, err := journalPage(ctx, since, until)
		if err != nil {
			return entries, "unavailable: journalctl or journal permissions"
		}
		for _, line := range strings.Split(string(raw), "\n") {
			var row struct {
				Message  string `json:"MESSAGE"`
				Unit     string `json:"_SYSTEMD_UNIT"`
				Priority string `json:"PRIORITY"`
				Stamp    string `json:"__REALTIME_TIMESTAMP"`
			}
			if json.Unmarshal([]byte(line), &row) != nil || row.Message == "" {
				continue
			}
			stamp, _ := strconv.ParseInt(row.Stamp, 10, 64)
			level := "INFO"
			p, _ := strconv.Atoi(row.Priority)
			if p <= 3 {
				level = "ERROR"
			} else if p == 4 {
				level = "WARN"
			}
			if time.UnixMicro(stamp).After(since) {
				entries = append(entries, logEntry{Time: time.UnixMicro(stamp).UTC().Format(time.RFC3339Nano), Level: level, Source: truncate(row.Unit, 128), Message: truncate(row.Message, 8192)})
			}
		}
		return entries, "available"
	}
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $start=[DateTimeOffset]::FromUnixTimeMilliseconds(%d).LocalDateTime; $end=[DateTimeOffset]::FromUnixTimeMilliseconds(%d).LocalDateTime; $rows=@(Get-WinEvent -FilterHashtable @{LogName='System','Application';StartTime=$start;EndTime=$end} -Oldest -MaxEvents 100 -ErrorAction SilentlyContinue | Where-Object {$_.TimeCreated -gt $start} | ForEach-Object {[pscustomobject]@{time=$_.TimeCreated.ToUniversalTime().ToString('o');level=[int]$_.Level;source=$_.ProviderName;message=$_.Message}}); ConvertTo-Json -Compress -InputObject $rows`, since.UnixMilli(), until.UnixMilli())
		raw, err := commandOutput(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		if err != nil {
			return entries, "unavailable: Windows Event Log permissions"
		}
		var rows []struct {
			Time    string
			Level   int
			Source  string
			Message string
		}
		if json.Unmarshal(raw, &rows) != nil {
			return entries, "unavailable: Windows Event Log response"
		}
		for _, v := range rows {
			level := "INFO"
			if v.Level <= 2 {
				level = "ERROR"
			} else if v.Level == 3 {
				level = "WARN"
			}
			entries = append(entries, logEntry{Time: v.Time, Level: level, Source: truncate(v.Source, 128), Message: truncate(v.Message, 8192)})
		}
		return entries, "available"
	}
	return entries, "unavailable: unsupported OS"
}
