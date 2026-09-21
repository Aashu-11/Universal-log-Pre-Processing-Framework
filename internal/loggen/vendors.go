// Package loggen implements the synthetic, per-vendor log line generators
// shared by tools/loggen (the live traffic generator) and tools/gen-corpus
// (which derives testdata/corpus + testdata/golden from the exact same
// generator, so parser fixtures can never drift from what the demo traffic
// generator actually produces).
package loggen

import (
	"fmt"
	"math/rand"
	"time"
)

// Vendor log line generators. These are hand-written to match each
// vendor's real wire syntax (field order, delimiters, syslog framing)
// closely enough to be a faithful parser-development target, but every
// value is synthetic/randomized — never copied from a real log — per
// CLAUDE.md's "no hardcoded/scraped sample data" rule. Phase 3's
// packs/<vendor>/... parsers are written against exactly this shape.

type Vendor string

const (
	VendorPaloAlto  Vendor = "paloalto"
	VendorFortinet  Vendor = "fortinet"
	VendorCiscoASA  Vendor = "cisco"
	VendorSonicWall Vendor = "sonicwall" // deliberately NOT in AllVendors/packs — see tools/gen-corpus's onboarding demo
)

var AllVendors = []Vendor{VendorPaloAlto, VendorFortinet, VendorCiscoASA}

// scanState tracks a synthetic port-scan in progress for --anomaly port-scan.
type scanState struct {
	active    bool
	srcIP     string
	dstIP     string
	nextPort  int
	remaining int
}

type Generator struct {
	rng           *rand.Rand
	scan          scanState
	malformedRate float64
}

func NewGenerator(seed int64) *Generator {
	return &Generator{rng: rand.New(rand.NewSource(seed))}
}

// SetMalformedRate arms a fraction (0-1) of generated events to carry a
// genuinely out-of-spec field — an out-of-range port or an invalid dotted
// IP — instead of anomaly-free traffic. This is not simulated failure: the
// bad value sits in an otherwise real, syntactically valid vendor log line,
// so it flows through the real parser (which just extracts whatever digits
// sit in the port/IP position) and is only caught downstream by
// internal/validate's real range/parse checks, landing in DLQ (or not) for
// the exact same reason a real misconfigured device's garbled log would.
func (g *Generator) SetMalformedRate(r float64) { g.malformedRate = r }

// badPort returns a value guaranteed outside 0-65535 — a corrupted or
// miscounted port field, or a NAT/proxy artifact a real device might emit.
func (g *Generator) badPort() int { return 65536 + g.rng.Intn(200000) }

// badIP returns a string that's syntactically IP-shaped (four dot-separated
// groups any regex/dissect pattern happily extracts) but numerically
// invalid — net.ParseIP correctly rejects it downstream in VALIDATE.
func (g *Generator) badIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", 256+g.rng.Intn(200), g.rng.Intn(255), g.rng.Intn(255), g.rng.Intn(255))
}

func (g *Generator) randIP(private bool) string {
	if private {
		return fmt.Sprintf("10.0.%d.%d", g.rng.Intn(255), g.rng.Intn(255))
	}
	return fmt.Sprintf("%d.%d.%d.%d", 20+g.rng.Intn(200), g.rng.Intn(255), g.rng.Intn(255), 1+g.rng.Intn(254))
}

func (g *Generator) randPort() int { return 1024 + g.rng.Intn(64511) }

// StartPortScan arms a synthetic port-scan burst: the next `n` generated
// events for any vendor will be TCP connections from one src IP to one dst
// IP across sequential destination ports — the signature a port-scan
// detector should catch, used by the Phase 9 ML/anomaly demo.
func (g *Generator) StartPortScan(n int) {
	g.scan = scanState{active: true, srcIP: g.randIP(false), dstIP: g.randIP(true), nextPort: 1, remaining: n}
}

func (g *Generator) nextScanPort() (srcIP, dstIP string, port int, ok bool) {
	if !g.scan.active || g.scan.remaining <= 0 {
		return "", "", 0, false
	}
	g.scan.remaining--
	port = g.scan.nextPort
	g.scan.nextPort++
	if g.scan.remaining == 0 {
		g.scan.active = false
	}
	return g.scan.srcIP, g.scan.dstIP, port, true
}

// Line generates one syntactically valid log line for vendor, including
// syslog PRI framing where the real device would send it. now is the event
// timestamp to embed.
func (g *Generator) Line(v Vendor, now time.Time) string {
	switch v {
	case VendorPaloAlto:
		return g.paloAltoTraffic(now)
	case VendorFortinet:
		return g.fortinetTraffic(now)
	case VendorCiscoASA:
		return g.ciscoASA302013(now)
	case VendorSonicWall:
		// Selectable explicitly (--vendors=sonicwall) once the onboarding
		// demo has actually published a parser for it — never included in
		// AllVendors, so it stays a genuinely never-seen shape for the next
		// person who runs the onboarding flow fresh.
		return g.SonicWallTraffic(now)
	default:
		return g.paloAltoTraffic(now)
	}
}

// SonicWallTraffic emits a SonicWall-style kv traffic log — deliberately
// NOT one of the parsers this repo ships, so it's a genuinely unknown
// source for Phase 8's onboarding-flow proof (`ulpfctl parser test` has
// never seen this shape, no pack references it).
func (g *Generator) SonicWallTraffic(now time.Time) string {
	srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
	sent := 100 + g.rng.Intn(50000)
	rcvd := 100 + g.rng.Intn(50000)
	actions := []string{"allow", "allow", "allow", "deny", "drop"}
	action := actions[g.rng.Intn(len(actions))]

	return fmt.Sprintf(
		`<134>id=firewall sn=C0EAE4%06X time="%s" fw=203.0.113.1 pri=6 c=1024 m=97 msg="Connection %s" n=1000 src=%s:%d:X0 dst=%s:%d:X1 srcMac=00:11:22:33:44:55 dstMac=aa:bb:cc:dd:ee:ff proto=tcp/https sent=%d rcvd=%d`,
		g.rng.Intn(0xFFFFFF), now.Format("2006-01-02 15:04:05"), action, srcIP, srcPort, dstIP, dstPort, sent, rcvd,
	)
}

func (g *Generator) srcDstPort() (srcIP, dstIP string, srcPort, dstPort int) {
	if sIP, dIP, port, ok := g.nextScanPort(); ok {
		return sIP, dIP, g.randPort(), port
	}
	commonPorts := []int{443, 443, 443, 80, 22, 53, 3389, 8443}
	srcIP, dstIP = g.randIP(false), g.randIP(true)
	srcPort, dstPort = g.randPort(), commonPorts[g.rng.Intn(len(commonPorts))]

	if g.malformedRate > 0 && g.rng.Float64() < g.malformedRate {
		switch g.rng.Intn(4) {
		case 0:
			srcPort = g.badPort()
		case 1:
			dstPort = g.badPort()
		case 2:
			srcIP = g.badIP()
		case 3:
			dstIP = g.badIP()
		}
	}
	return srcIP, dstIP, srcPort, dstPort
}

// paloAltoTraffic emits a PAN-OS style TRAFFIC log: syslog header + a CSV
// body. Real PAN-OS TRAFFIC logs run to ~60 columns; this is a well-defined
// 15-column subset that packs/paloalto/panos/traffic.yaml is written to
// extract exactly — kept deliberately small so the generator and the parser
// pack stay trivially in lockstep instead of drifting on 60 rarely-used
// columns neither side actually exercises.
//
// Columns: receive_time,serial,type,subtype,src,dst,rule,app,from_zone,
//
//	to_zone,session_id,src_port,dst_port,protocol,action,
//	bytes_sent,bytes_received,packets,elapsed_time,device_name
func (g *Generator) paloAltoTraffic(now time.Time) string {
	srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
	sent := 100 + g.rng.Intn(50000)
	rcvd := 100 + g.rng.Intn(50000)
	actions := []string{"allow", "allow", "allow", "deny", "drop"}
	action := actions[g.rng.Intn(len(actions))]
	rule := fmt.Sprintf("rule-%d", 1+g.rng.Intn(20))
	ts := now.Format("2006/01/02 15:04:05")

	return fmt.Sprintf(
		"<14>%s pa-edge-01 1,%s,001606001116,TRAFFIC,end,%s,%s,%s,ssl,untrust,trust,%d,%d,%d,tcp,%s,%d,%d,20,10,pa-edge-01",
		now.Format("Jan  2 15:04:05"), ts, srcIP, dstIP, rule,
		10000+g.rng.Intn(50000), srcPort, dstPort, action, sent, rcvd,
	)
}

// fortinetTraffic emits a FortiGate key=value style traffic log.
func (g *Generator) fortinetTraffic(now time.Time) string {
	srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
	sent := 100 + g.rng.Intn(50000)
	rcvd := 100 + g.rng.Intn(50000)
	actions := []string{"accept", "accept", "accept", "deny", "close"}
	action := actions[g.rng.Intn(len(actions))]
	services := []string{"HTTPS", "HTTP", "DNS", "SSH", "RDP"}
	service := services[g.rng.Intn(len(services))]

	return fmt.Sprintf(
		`<189>date=%s time=%s devname="FG-Edge" devid="FG100E1234" logid="0000000013" type="traffic" subtype="forward" level="notice" vd="root" eventtime=%d srcip=%s srcport=%d srcintf="wan1" dstip=%s dstport=%d dstintf="lan" action="%s" policyid=%d proto=6 service="%s" sentbyte=%d rcvdbyte=%d duration=%d`,
		now.Format("2006-01-02"), now.Format("15:04:05"), now.Unix(), srcIP, srcPort, dstIP, dstPort,
		action, 1+g.rng.Intn(20), service, sent, rcvd, 1+g.rng.Intn(300),
	)
}

// ciscoASA302013 emits a Cisco ASA %ASA-6-302013 "Built ... connection"
// message.
func (g *Generator) ciscoASA302013(now time.Time) string {
	srcIP, dstIP, srcPort, dstPort := g.srcDstPort()
	connID := 100000 + g.rng.Intn(900000)
	directions := []string{"inbound", "outbound"}
	direction := directions[g.rng.Intn(len(directions))]

	return fmt.Sprintf(
		"<166>%s asa-fw-01 : %%ASA-6-302013: Built %s TCP connection %d for outside:%s/%d (%s/%d) to inside:%s/%d (%s/%d)",
		now.Format("Jan 02 2006 15:04:05"), direction, connID, srcIP, srcPort, srcIP, srcPort, dstIP, dstPort, dstIP, dstPort,
	)
}
