package identify_test

import (
	"path/filepath"
	"testing"

	"github.com/ulpf/ulpf/internal/identify"
	"github.com/ulpf/ulpf/internal/parse"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

func newTestResolver(t *testing.T) *identify.Resolver {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		t.Fatalf("load packs: %v", err)
	}
	return identify.NewResolver(registry)
}

func TestResolveBySignature(t *testing.T) {
	r := newTestResolver(t)
	raw := []byte("<166>Sep 07 2026 14:02:11 asa-fw-01 : %ASA-6-302013: Built inbound TCP connection 1 for outside:1.2.3.4/1 (1.2.3.4/1) to inside:5.6.7.8/2 (5.6.7.8/2)")

	res := r.Resolve("syslog-tcp", "203.0.113.9", raw)
	if res.Tier != "signature" {
		t.Fatalf("tier = %q, want signature", res.Tier)
	}
	if res.Plan.Metadata.ID != "cisco.asa.302013" {
		t.Fatalf("resolved to %q, want cisco.asa.302013", res.Plan.Metadata.ID)
	}
	if res.Shape != identify.ShapeSyslog {
		t.Errorf("shape = %q, want syslog", res.Shape)
	}
}

func TestResolveUnknownNeverPanicsAndReturnsUnknownTier(t *testing.T) {
	r := newTestResolver(t)
	res := r.Resolve("syslog-tcp", "203.0.113.9", []byte("totally unrecognized garbage log line"))
	if res.Tier != "unknown" {
		t.Fatalf("tier = %q, want unknown", res.Tier)
	}
	if res.Plan != nil {
		t.Fatalf("expected nil plan for unknown source")
	}
}

func TestBindingTierOverridesSignatureMatch(t *testing.T) {
	r := newTestResolver(t)
	// Bind this peer explicitly to the Cisco parser even though we'll feed
	// it a line whose *own* signature would normally match PAN-OS — the
	// binding tier must win, since an operator has pinned this source.
	r.Bind("syslog-tcp", "10.5.5.5", "cisco.asa.302013")

	panosLine := []byte("<14>Sep  7 14:02:11 pa-edge-01 1,2026/09/07 14:02:11,001,TRAFFIC,end,1.2.3.4,5.6.7.8,r1,ssl,untrust,trust,1,2,3,tcp,allow,4,5,6,7,host")
	res := r.Resolve("syslog-tcp", "10.5.5.5", panosLine)
	if res.Tier != "binding" {
		t.Fatalf("tier = %q, want binding", res.Tier)
	}
	if res.Plan.Metadata.ID != "cisco.asa.302013" {
		t.Fatalf("binding did not override signature match: got %q", res.Plan.Metadata.ID)
	}
}
