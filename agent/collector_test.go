//go:build integration

package main

import (
	"context"
	"testing"
)

func TestCollectorReturnsHostAndRealMemory(t *testing.T) {
	c := newCollector()
	r, err := c.collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.OS == "" || r.Hardware.CPUCores < 1 || r.Hardware.MemoryGB <= 0 {
		t.Fatalf("host missing: %+v", r.Hardware)
	}
	if r.Sample.Memory <= 0 || r.Sample.Memory > 100 {
		t.Fatalf("invalid RAM: %+v", r.Sample)
	}
	if len(r.Processes) == 0 || len(r.Disks) == 0 {
		t.Fatal("no processes or disks")
	}
}
