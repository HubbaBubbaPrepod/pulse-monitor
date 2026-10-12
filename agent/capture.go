package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type packetBuffer struct {
	mu      sync.Mutex
	rows    []packet
	message string
}

func (p *packetBuffer) snapshot() []packet {
	p.mu.Lock()
	defer p.mu.Unlock()
	rows := append([]packet{}, p.rows...)
	p.rows = nil
	return rows
}
func (p *packetBuffer) status() string { p.mu.Lock(); defer p.mu.Unlock(); return p.message }
func startCapture(ctx context.Context, iface string) (*packetBuffer, func()) {
	buffer := &packetBuffer{rows: []packet{}, message: "disabled: enable -capture with an installed TShark"}
	if iface == "" {
		return buffer, func() {}
	}
	path, err := exec.LookPath("tshark")
	if err != nil {
		buffer.message = "unavailable: TShark is not installed"
		return buffer, func() {}
	}
	captureCtx, cancel := context.WithCancel(ctx)
	args := []string{"-l", "-n", "-p", "-i", iface, "-T", "fields", "-E", "separator=|", "-E", "occurrence=f"}
	for _, field := range []string{"frame.time_epoch", "ip.src", "ipv6.src", "tcp.srcport", "udp.srcport", "ip.dst", "ipv6.dst", "tcp.dstport", "udp.dstport", "_ws.col.Protocol", "frame.len", "tcp.flags", "_ws.col.Info"} {
		args = append(args, "-e", field)
	}
	cmd := exec.CommandContext(captureCtx, path, args...)
	configureCommand(cmd)
	cmd.Stderr = &limitedBuffer{}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		buffer.message = "unavailable: capture pipe"
		return buffer, func() {}
	}
	if err := cmd.Start(); err != nil {
		cancel()
		buffer.message = "unavailable: capture permissions or driver"
		return buffer, func() {}
	}
	buffer.message = "available: metadata sample, up to 256 packets per report"
	done := make(chan struct{})
	local := map[string]bool{}
	addresses, _ := net.InterfaceAddrs()
	for _, v := range addresses {
		ip, _, err := net.ParseCIDR(v.String())
		if err == nil {
			local[ip.String()] = true
		}
	}
	go func() {
		defer close(done)
		scan := bufio.NewScanner(stdout)
		scan.Buffer(make([]byte, 4096), 64*1024)
		sequence := 0
		for scan.Scan() {
			cols := strings.Split(scan.Text(), "|")
			if len(cols) < 13 {
				continue
			}
			source := cols[1]
			if source == "" {
				source = cols[2]
			}
			dest := cols[5]
			if dest == "" {
				dest = cols[6]
			}
			srcPort := cols[3]
			if srcPort == "" {
				srcPort = cols[4]
			}
			dstPort := cols[7]
			if dstPort == "" {
				dstPort = cols[8]
			}
			seconds, _ := strconv.ParseFloat(cols[0], 64)
			length, _ := strconv.Atoi(cols[10])
			direction := "in"
			if local[source] {
				direction = "out"
			}
			sequence++
			p := packet{ID: fmt.Sprintf("capture-%d", sequence), Time: time.Unix(0, int64(seconds*1e9)).UTC().Format(time.RFC3339Nano), Direction: direction, Source: source, Destination: dest, Protocol: cols[9], Length: length, Flags: cols[11], Process: "—", Summary: truncate(strings.Join(cols[12:], "|"), 512)}
			if srcPort != "" {
				p.Source = net.JoinHostPort(source, srcPort)
			}
			if dstPort != "" {
				p.Destination = net.JoinHostPort(dest, dstPort)
			}
			buffer.mu.Lock()
			if len(buffer.rows) < 256 {
				buffer.rows = append(buffer.rows, p)
			}
			buffer.mu.Unlock()
		}
		if scan.Err() != nil {
			cancel()
		}
		err := cmd.Wait()
		buffer.mu.Lock()
		if err != nil && captureCtx.Err() == nil {
			buffer.message = "unavailable: capture stopped; check permissions and driver"
		}
		buffer.mu.Unlock()
	}()
	return buffer, func() { cancel(); <-done }
}
