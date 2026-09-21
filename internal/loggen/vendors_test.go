package loggen

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestSonicWallNeverInAllVendors is a regression guard for D-010: SonicWall
// must stay out of AllVendors so it remains a genuinely never-seen shape
// for whoever next runs the onboarding demo fresh, even though Line() and
// tools/loggen's --vendors flag both now accept it explicitly (used once
// this repo's own onboarding demo already published a parser for it).
func TestSonicWallNeverInAllVendors(t *testing.T) {
	for _, v := range AllVendors {
		if v == VendorSonicWall {
			t.Fatal("VendorSonicWall must never be included in AllVendors")
		}
	}
}

// TestLineSonicWallProducesRealSonicWallShape proves Generator.Line's new
// VendorSonicWall case actually delegates to SonicWallTraffic rather than
// falling through to the default (paloAltoTraffic) case.
func TestLineSonicWallProducesRealSonicWallShape(t *testing.T) {
	g := NewGenerator(1)
	line := g.Line(VendorSonicWall, time.Now())
	if !strings.Contains(line, "id=firewall") || !strings.Contains(line, "srcMac=") {
		t.Fatalf("Line(VendorSonicWall, ...) = %q, does not look like SonicWallTraffic's output", line)
	}
}

// TestMalformedRateZeroProducesNoOutOfRangeValues guards the default
// (demo-traffic) path: with malformed_rate unset, srcDstPort must never
// return a port outside 0-65535 or an IP net.ParseIP rejects.
func TestMalformedRateZeroProducesNoOutOfRangeValues(t *testing.T) {
	g := NewGenerator(42)
	for i := 0; i < 2000; i++ {
		srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
		if srcPort < 0 || srcPort > 65535 {
			t.Fatalf("srcPort out of range with malformedRate=0: %d", srcPort)
		}
		if dstPort < 0 || dstPort > 65535 {
			t.Fatalf("dstPort out of range with malformedRate=0: %d", dstPort)
		}
		if net.ParseIP(srcIP) == nil {
			t.Fatalf("srcIP does not parse with malformedRate=0: %q", srcIP)
		}
		if net.ParseIP(dstIP) == nil {
			t.Fatalf("dstIP does not parse with malformedRate=0: %q", dstIP)
		}
	}
}

// TestMalformedRateProducesGenuineViolations is a regression test for the
// DLQ-seeding mechanism: at malformedRate=1 (always), every generated
// (srcIP,dstIP,srcPort,dstPort) quad must contain at least one field that
// would actually fail internal/validate's real checks (out-of-range port,
// or an IP net.ParseIP rejects) — proving the "malformed" traffic is a
// genuine validation failure, not just cosmetically different data.
func TestMalformedRateProducesGenuineViolations(t *testing.T) {
	g := NewGenerator(7)
	g.SetMalformedRate(1.0)

	violations := 0
	for i := 0; i < 500; i++ {
		srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
		bad := srcPort < 0 || srcPort > 65535 ||
			dstPort < 0 || dstPort > 65535 ||
			net.ParseIP(srcIP) == nil ||
			net.ParseIP(dstIP) == nil
		if bad {
			violations++
		}
	}
	if violations != 500 {
		t.Fatalf("malformedRate=1.0: got %d/500 genuinely invalid quads, want 500", violations)
	}
}

// TestMalformedLineStillParsesAsExpectedShape proves the corrupted field
// sits inside an otherwise well-formed vendor line — the point of this
// mechanism is that the parser succeeds and extracts the bad value (it's
// VALIDATE's job to catch it), not that the line becomes unparseable
// garbage. A Fortinet line's srcport=NNNN token must still be present and
// numeric, even when malformed-rate corrupted it out of port range.
func TestMalformedLineStillParsesAsExpectedShape(t *testing.T) {
	g := NewGenerator(99)
	g.SetMalformedRate(1.0)

	for i := 0; i < 50; i++ {
		line := g.fortinetTraffic(time.Now())
		idx := strings.Index(line, "srcport=")
		if idx == -1 {
			t.Fatalf("line missing srcport= token: %s", line)
		}
		rest := line[idx+len("srcport="):]
		end := strings.IndexByte(rest, ' ')
		if end == -1 {
			end = len(rest)
		}
		if _, err := strconv.Atoi(rest[:end]); err != nil {
			t.Fatalf("srcport value not numeric even when malformed: %q in line %s", rest[:end], line)
		}
	}
}
