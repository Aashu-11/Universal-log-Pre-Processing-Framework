// Command loggen emits realistic, syntactically correct logs for ULPF's
// target vendors at a configurable EPS, generated from documented format
// templates — never copied from real logs — through the real ingest
// protocols (UDP/TCP/HTTP), the way every demo in this repo gets its data.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ulpf/ulpf/internal/loggen"
)

func main() {
	vendorsFlag := flag.String("vendors", "all", "comma-separated vendors (paloalto,fortinet,cisco) or 'all'")
	eps := flag.Int("eps", 1000, "target events per second")
	duration := flag.Duration("duration", 30*time.Second, "how long to run")
	proto := flag.String("proto", "udp", "udp | tcp | http")
	host := flag.String("host", "127.0.0.1", "collector host")
	port := flag.Int("port", 5514, "collector port (5514 udp/tcp default, 8088 for http)")
	anomaly := flag.String("anomaly", "none", "none | port-scan")
	seed := flag.Int64("seed", time.Now().UnixNano(), "PRNG seed")
	flag.Parse()

	vendors := parseVendors(*vendorsFlag)
	if len(vendors) == 0 {
		fmt.Fprintln(os.Stderr, "loggen: no valid vendors selected")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	var raw Sender
	var err error
	switch *proto {
	case "udp":
		raw, err = newUDPSender(addr)
	case "tcp":
		raw, err = newTCPSender(addr)
	case "http":
		raw, err = newHTTPSender(addr)
	default:
		fmt.Fprintf(os.Stderr, "loggen: unknown --proto %q\n", *proto)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "loggen: connect to %s (%s): %v\n", addr, *proto, err)
		os.Exit(1)
	}

	var sentCount, errCount atomic.Int64
	sender := &countingSender{inner: raw, sent: &sentCount, errs: &errCount}
	defer sender.Close()

	gen := loggen.NewGenerator(*seed)
	if *anomaly == "port-scan" {
		gen.StartPortScan(500)
	}

	fmt.Printf("loggen: vendors=%v eps=%d duration=%s proto=%s target=%s anomaly=%s\n",
		vendors, *eps, *duration, *proto, addr, *anomaly)

	// Pace by wall-clock deficit rather than one OS timer tick per event:
	// at *eps above a few thousand, a per-event time.Ticker is limited by
	// the platform's timer resolution (Windows in particular coalesces
	// sub-millisecond ticks, capping real throughput around ~2000 eps
	// regardless of the requested rate). Instead, wake on a coarse,
	// timer-friendly cadence, compute how many events *should* have been
	// sent by elapsed wall-clock time at the target rate, and send that
	// many in a tight loop — bursty at the microsecond scale, accurate at
	// the second scale, and immune to OS timer granularity.
	const pacingInterval = 5 * time.Millisecond
	pacer := time.NewTicker(pacingInterval)
	defer pacer.Stop()

	statusTicker := time.NewTicker(1 * time.Second)
	defer statusTicker.Stop()

	deadline := time.After(*duration)
	start := time.Now()
	i := 0
	targetSent := 0

loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-statusTicker.C:
			elapsed := time.Since(start).Seconds()
			fmt.Printf("loggen: t=%.0fs sent=%d errors=%d rate=%.0f eps\n",
				elapsed, sentCount.Load(), errCount.Load(), float64(sentCount.Load())/elapsed)
		case <-pacer.C:
			targetSent = int(time.Since(start).Seconds() * float64(*eps))
			for i < targetSent {
				v := vendors[i%len(vendors)]
				line := gen.Line(v, time.Now())
				if err := sender.Send(line); err != nil {
					fmt.Fprintf(os.Stderr, "loggen: send error: %v\n", err)
				}
				i++
			}
		}
	}

	elapsed := time.Since(start).Seconds()
	fmt.Printf("loggen: DONE sent=%d errors=%d elapsed=%.1fs avg_rate=%.0f eps\n",
		sentCount.Load(), errCount.Load(), elapsed, float64(sentCount.Load())/elapsed)
}

func parseVendors(s string) []loggen.Vendor {
	if s == "all" {
		return loggen.AllVendors
	}
	var out []loggen.Vendor
	for _, part := range strings.Split(s, ",") {
		switch loggen.Vendor(strings.TrimSpace(part)) {
		case loggen.VendorPaloAlto:
			out = append(out, loggen.VendorPaloAlto)
		case loggen.VendorFortinet:
			out = append(out, loggen.VendorFortinet)
		case loggen.VendorCiscoASA:
			out = append(out, loggen.VendorCiscoASA)
		}
	}
	return out
}
