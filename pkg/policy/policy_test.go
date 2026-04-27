package policy

import (
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		in    string
		want  uint64
		isErr bool
	}{
		{"1024", 1024, false},
		{"1k", 1024, false},
		{"512m", 512 << 20, false},
		{"2g", 2 << 30, false},
		{"1T", 1 << 40, false},
		{"  256M  ", 256 << 20, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1x", 0, true},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.isErr {
			if err == nil {
				t.Errorf("parseSize(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSize(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestCapValidate(t *testing.T) {
	good := Cap{Mem: "1g", Memswap: "1g", Pids: 1024, OOMAdj: 500}
	if err := good.Validate(); err != nil {
		t.Errorf("good cap should validate: %v", err)
	}

	cases := []struct {
		name string
		cap  Cap
		err  string
	}{
		{"zero mem", Cap{Mem: "0", Memswap: "0", Pids: 1024, OOMAdj: 500}, "mem"},
		{"swap < mem", Cap{Mem: "2g", Memswap: "1g", Pids: 1024, OOMAdj: 500}, "memswap"},
		{"low pids", Cap{Mem: "1g", Memswap: "1g", Pids: 32, OOMAdj: 500}, "pids_limit"},
		{"zero oom", Cap{Mem: "1g", Memswap: "1g", Pids: 1024, OOMAdj: 0}, "oom_score_adj"},
		{"negative oom", Cap{Mem: "1g", Memswap: "1g", Pids: 1024, OOMAdj: -100}, "oom_score_adj"},
	}
	for _, c := range cases {
		err := c.cap.Validate()
		if err == nil {
			t.Errorf("%s: expected error containing %q, got nil", c.name, c.err)
			continue
		}
		if !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: expected error containing %q, got %q", c.name, c.err, err.Error())
		}
	}
}

func TestPolicyCapFor(t *testing.T) {
	p := Default()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy fails its own validation: %v", err)
	}

	cases := []struct {
		svc      string
		wantMem  string
		wantPids int
	}{
		{"postgres", "4g", 1024},
		{"helixagent-postgres", "4g", 1024},
		{"redis", "1g", 1024},
		{"redis-cache", "1g", 1024},
		{"ollama", "12g", 2048},
		{"vllm-runner", "12g", 2048},
		{"mcp-fetch", "1g", 1024},
		{"mcp-puppeteer", "3g", 4096},
		{"mcp-playwright-headless", "3g", 4096},
		{"mcp-stable-diffusion-cpu", "4g", 1024},
		{"helixagent", "4g", 2048},
		{"helixllm-server", "4g", 2048},
		{"sonarqube", "8g", 2048},
		{"unknown-service-name", "2g", 1024}, // default
		{"POSTGRES", "4g", 1024},             // case-insensitive
	}
	for _, c := range cases {
		got := p.CapFor(c.svc)
		if got.Mem != c.wantMem {
			t.Errorf("CapFor(%q).Mem = %q, want %q", c.svc, got.Mem, c.wantMem)
		}
		if got.Pids != c.wantPids {
			t.Errorf("CapFor(%q).Pids = %d, want %d", c.svc, got.Pids, c.wantPids)
		}
		if got.OOMAdj <= 0 {
			t.Errorf("CapFor(%q).OOMAdj = %d, must be >0", c.svc, got.OOMAdj)
		}
		mem, swap, err := got.Bytes()
		if err != nil {
			t.Errorf("CapFor(%q).Bytes() failed: %v", c.svc, err)
		}
		if swap < mem {
			t.Errorf("CapFor(%q): swap %d < mem %d", c.svc, swap, mem)
		}
	}
}

func TestPolicyOrderingFirstMatchWins(t *testing.T) {
	// `mcp-puppeteer` matches both `mcp-puppeteer*` (3g, 4096 pids) and
	// `mcp-*` (1g, 1024 pids). The browser-specific rule sits earlier and
	// must win.
	p := Default()
	got := p.CapFor("mcp-puppeteer-1.2")
	if got.Mem != "3g" {
		t.Errorf("most-specific rule lost ordering: got %s, want 3g", got.Mem)
	}
}

func TestEveryDefaultRuleValidates(t *testing.T) {
	p := Default()
	if err := p.Validate(); err != nil {
		t.Fatalf("Default() policy invalid: %v", err)
	}
	if len(p.Rules) < 30 {
		t.Errorf("expected at least 30 default rules, got %d", len(p.Rules))
	}
}

func TestCapBytes(t *testing.T) {
	c := Cap{Mem: "1g", Memswap: "2g", Pids: 1024, OOMAdj: 500}
	mem, swap, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if mem != 1<<30 {
		t.Errorf("mem = %d, want %d", mem, 1<<30)
	}
	if swap != 2<<30 {
		t.Errorf("swap = %d, want %d", swap, 2<<30)
	}

	bad := Cap{Mem: "2g", Memswap: "1g", Pids: 1024, OOMAdj: 500}
	if _, _, err := bad.Bytes(); err == nil {
		t.Errorf("Bytes should reject memswap < mem")
	}
}
