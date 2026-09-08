// Package telemetry defines the Prometheus metrics shared across the data
// plane (ingest, vault, parse, enrich, sink).
package telemetry

import "github.com/prometheus/client_golang/prometheus"

// Metrics is the full set of data-plane metrics, all registered against a
// caller-supplied registry so tests can use their own isolated registry
// instead of the global default.
type Metrics struct {
	EventsReceivedTotal *prometheus.CounterVec
	UDPDropsTotal       prometheus.Counter
	IngestBytesTotal    *prometheus.CounterVec
	SpoolDepth          *prometheus.GaugeVec
	VaultWriteDuration  prometheus.Histogram
	ParseDuration       *prometheus.HistogramVec
	DLQTotal            *prometheus.CounterVec
	EnrichDuration      *prometheus.HistogramVec
}

// New registers and returns a fresh Metrics set on reg.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		EventsReceivedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ulpf_events_received_total",
			Help: "Total events received, by listener and peer.",
		}, []string{"listener", "peer"}),

		UDPDropsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ulpf_udp_drops_total",
			Help: "Total UDP datagrams dropped because the ingest buffer was full. UDP loss must be visible, never silent.",
		}),

		IngestBytesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ulpf_ingest_bytes_total",
			Help: "Total raw bytes received, by listener.",
		}, []string{"listener"}),

		SpoolDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ulpf_spool_depth",
			Help: "Current number of events sitting in the disk-backed overflow spool, by listener.",
		}, []string{"listener"}),

		VaultWriteDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "ulpf_vault_write_duration_seconds",
			Help:    "Latency of Vault.WriteBatch calls.",
			Buckets: prometheus.DefBuckets,
		}),

		ParseDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ulpf_parse_duration_seconds",
			Help:    "Per-event parse duration, by parser id.",
			Buckets: prometheus.ExponentialBuckets(0.00005, 2, 16),
		}, []string{"parser_id"}),

		DLQTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ulpf_dlq_total",
			Help: "Total events sent to the dead-letter queue, by reason.",
		}, []string{"reason"}),

		EnrichDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ulpf_enrich_duration_seconds",
			Help:    "Per-enricher latency.",
			Buckets: prometheus.ExponentialBuckets(0.000005, 2, 16),
		}, []string{"enricher"}),
	}

	reg.MustRegister(
		m.EventsReceivedTotal, m.UDPDropsTotal, m.IngestBytesTotal, m.SpoolDepth,
		m.VaultWriteDuration, m.ParseDuration, m.DLQTotal, m.EnrichDuration,
	)
	return m
}
