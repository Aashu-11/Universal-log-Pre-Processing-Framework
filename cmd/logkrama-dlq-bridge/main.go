// Command logkrama-dlq-bridge consumes the logkrama.dlq Kafka topic and forwards
// each entry to the control plane's POST /v1/dlq, populating the
// meta.dlq_events table (and so the console's DLQ page) with the real
// reason(s) route.Router attached — see internal/sink/kafka.Sink.WriteDLQ
// and internal/route.DLQSink. Previously documented as "not yet wired end
// to end since it needs a reachable Kafka broker to test against"; this is
// that wiring, now that one is.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kg "github.com/segmentio/kafka-go"

	sinkkafka "github.com/logkrama/logkrama/internal/sink/kafka"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "logkrama-dlq-bridge: %v\n", err)
		os.Exit(1)
	}
}

type dlqEventIn struct {
	EventID  string         `json:"event_id"`
	RawRef   map[string]any `json:"raw_ref"`
	Reason   string         `json:"reason"`
	ParserID *string        `json:"parser_id,omitempty"`
}

func run() error {
	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:29092"), ",")
	topic := envOr("KAFKA_TOPIC_DLQ", "logkrama.dlq")
	controlPlaneURL := envOr("LOGKRAMA_CONTROL_PLANE_URL", "http://localhost:8000")

	reader := kg.NewReader(kg.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "logkrama-dlq-bridge",
	})
	defer reader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	fmt.Printf("logkrama-dlq-bridge: bridging %s -> %s/v1/dlq\n", topic, controlPlaneURL)

	forwarded := 0
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Printf("logkrama-dlq-bridge: shutting down, forwarded %d\n", forwarded)
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var dm sinkkafka.DLQMessage
		if err := json.Unmarshal(msg.Value, &dm); err != nil {
			fmt.Fprintf(os.Stderr, "logkrama-dlq-bridge: bad message, skipping: %v\n", err)
			continue
		}

		body := dlqEventIn{
			EventID: dm.EventID,
			RawRef: map[string]any{
				"segment_id": dm.RawSegmentID,
				"offset":     dm.RawOffset,
				"length":     dm.RawLength,
				"sha256":     dm.RawSHA256,
			},
			Reason: strings.Join(dm.DLQReasons, ","),
		}
		if dm.LineageParserID != "" {
			body.ParserID = &dm.LineageParserID
		}
		if body.Reason == "" {
			body.Reason = "unspecified"
		}

		if err := postDLQEvent(ctx, client, controlPlaneURL, body); err != nil {
			fmt.Fprintf(os.Stderr, "logkrama-dlq-bridge: forward %s failed: %v\n", dm.EventID, err)
			continue
		}
		forwarded++
	}
}

func postDLQEvent(ctx context.Context, client *http.Client, baseURL string, body dlqEventIn) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/dlq", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("control plane returned %d", resp.StatusCode)
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
