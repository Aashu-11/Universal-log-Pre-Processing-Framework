// Command gen-enrichment builds every offline enrichment dataset under
// enrichment/ at BUILD time — the only time this repo's enrichment data is
// ever generated. The running system (internal/enrich) only ever reads
// these files; it never fetches anything at runtime, which is what makes
// air-gapped deployment possible. See docs/DECISIONS.md D-002 for why this
// is a synthetic CIDR-range dataset rather than a downloaded MaxMind/DB-IP
// .mmdb binary.
//
// Everything here is deterministically generated from a fixed seed (never
// scraped from a real feed, per CLAUDE.md), so re-running this is a no-op
// diff unless the generation logic itself changes.
package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

const outDir = "enrichment"

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal("mkdir %s: %v", outDir, err)
	}

	checksums := map[string]string{}
	write := func(name string, rows [][]string, header []string) {
		path := filepath.Join(outDir, name)
		writeCSV(path, header, rows)
		checksums[name] = sha256File(path)
		fmt.Printf("gen-enrichment: %-20s %5d rows\n", name, len(rows))
	}

	write("geoip.csv", genGeoIP(), []string{"cidr", "country", "city"})
	write("asn.csv", genASN(), []string{"cidr", "asn", "as_org"})
	write("iocs.csv", genIOCs(), []string{"indicator", "type", "category", "confidence"})
	write("mitre-map.csv", genMitreMap(), []string{"signature_id", "tactic", "technique_id", "technique_name"})
	write("assets.csv", genAssets(), []string{"ip", "asset_id", "owner", "criticality", "environment"})
	write("identities.csv", genIdentities(), []string{"username", "department", "manager", "employee_id"})

	writeManifest(checksums)
}

func genGeoIP() [][]string {
	countries := []struct{ code, city string }{
		{"US", "Ashburn"}, {"US", "San Jose"}, {"DE", "Frankfurt"}, {"GB", "London"},
		{"SG", "Singapore"}, {"JP", "Tokyo"}, {"AU", "Sydney"}, {"BR", "Sao Paulo"},
		{"IN", "Mumbai"}, {"FR", "Paris"}, {"NL", "Amsterdam"}, {"CA", "Toronto"},
	}
	rng := rand.New(rand.NewSource(1))
	var rows [][]string
	// Every /8 from 1..223 (skipping private/reserved ranges), one country
	// each — coarse but deterministic and sufficient for a demo dataset.
	for i := 1; i <= 223; i++ {
		if isPrivateOrReservedFirstOctet(i) {
			continue
		}
		c := countries[rng.Intn(len(countries))]
		rows = append(rows, []string{fmt.Sprintf("%d.0.0.0/8", i), c.code, c.city})
	}
	return rows
}

func genASN() [][]string {
	orgs := []struct {
		asn int
		org string
	}{
		{15169, "Example Cloud LLC"}, {16509, "Example AWS-like Networks"},
		{8075, "Example Azure-like Compute"}, {13335, "Example CDN Networks"},
		{32934, "Example Social Platform"}, {714, "Example ISP Group"},
		{7018, "Example Tier1 Carrier"}, {6939, "Example Hosting Co"},
	}
	rng := rand.New(rand.NewSource(2))
	var rows [][]string
	for i := 1; i <= 223; i++ {
		if isPrivateOrReservedFirstOctet(i) {
			continue
		}
		o := orgs[rng.Intn(len(orgs))]
		rows = append(rows, []string{fmt.Sprintf("%d.0.0.0/8", i), fmt.Sprintf("%d", o.asn), o.org})
	}
	return rows
}

// genIOCs generates 10,000 synthetic indicators plus a handful of
// deliberately well-known values loggen's port-scan anomaly mode is
// documented to reuse, so the demo's IOC-match panel always has something
// to light up.
func genIOCs() [][]string {
	rng := rand.New(rand.NewSource(3))
	rows := [][]string{
		{"203.0.113.66", "ip", "c2", "90"},
		{"198.51.100.23", "ip", "scanner", "75"},
		{"evil-domain-example.test", "domain", "phishing", "85"},
	}
	types := []string{"ip", "domain", "hash"}
	categories := []string{"c2", "scanner", "phishing", "malware", "botnet"}
	for i := 0; i < 10000; i++ {
		typ := types[rng.Intn(len(types))]
		var indicator string
		switch typ {
		case "ip":
			indicator = fmt.Sprintf("%d.%d.%d.%d", 20+rng.Intn(200), rng.Intn(255), rng.Intn(255), 1+rng.Intn(254))
		case "domain":
			indicator = fmt.Sprintf("synthetic-ioc-%d.test", i)
		default:
			b := make([]byte, 32)
			rng.Read(b)
			indicator = hex.EncodeToString(b)
		}
		rows = append(rows, []string{
			indicator, typ, categories[rng.Intn(len(categories))],
			fmt.Sprintf("%d", 40+rng.Intn(60)),
		})
	}
	return rows
}

func genMitreMap() [][]string {
	return [][]string{
		{"2210000", "TA0001", "T1190", "Exploit Public-Facing Application"},
		{"2210001", "TA0043", "T1595", "Active Scanning"},
		{"2210002", "TA0011", "T1071", "Application Layer Protocol"},
		{"2210003", "TA0040", "T1499", "Endpoint Denial of Service"},
		{"2210004", "TA0006", "T1110", "Brute Force"},
		{"2210005", "TA0008", "T1021", "Remote Services"},
	}
}

func genAssets() [][]string {
	rng := rand.New(rand.NewSource(4))
	owners := []string{"platform-team", "security-team", "network-team", "app-team-checkout", "app-team-auth"}
	criticalities := []string{"low", "medium", "high", "critical"}
	envs := []string{"prod", "staging", "dev"}
	var rows [][]string
	for i := 0; i < 200; i++ {
		ip := fmt.Sprintf("10.0.%d.%d", rng.Intn(255), rng.Intn(255))
		rows = append(rows, []string{
			ip, fmt.Sprintf("asset-%04d", i), owners[rng.Intn(len(owners))],
			criticalities[rng.Intn(len(criticalities))], envs[rng.Intn(len(envs))],
		})
	}
	return rows
}

func genIdentities() [][]string {
	rng := rand.New(rand.NewSource(5))
	depts := []string{"engineering", "finance", "sales", "security", "hr"}
	var rows [][]string
	for i := 0; i < 200; i++ {
		rows = append(rows, []string{
			fmt.Sprintf("user%04d", i), depts[rng.Intn(len(depts))],
			fmt.Sprintf("manager%02d", rng.Intn(20)), fmt.Sprintf("E%05d", 10000+i),
		})
	}
	return rows
}

// isPrivateOrReservedFirstOctet skips RFC1918/loopback/link-local/reserved
// first octets so the synthetic geo/ASN datasets don't claim a "country"
// for private address space, which internal/enrich/cidr.go classifies
// separately anyway.
func isPrivateOrReservedFirstOctet(o int) bool {
	switch {
	case o == 10, o == 127, o == 169, o == 172, o == 192, o >= 224:
		return true
	default:
		return false
	}
}

func writeCSV(path string, header []string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		fatal("create %s: %v", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		fatal("write header %s: %v", path, err)
	}
	for _, r := range rows {
		if err := w.Write(r); err != nil {
			fatal("write row %s: %v", path, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fatal("flush %s: %v", path, err)
	}
}

func sha256File(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s for checksum: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeManifest(checksums map[string]string) {
	path := filepath.Join(outDir, "MANIFEST.sha256")
	f, err := os.Create(path)
	if err != nil {
		fatal("create manifest: %v", err)
	}
	defer f.Close()

	names := make([]string, 0, len(checksums))
	for n := range checksums {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(f, "%s  %s\n", checksums[n], n)
	}
	fmt.Printf("gen-enrichment: wrote %s\n", path)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-enrichment: "+format+"\n", args...)
	os.Exit(1)
}
