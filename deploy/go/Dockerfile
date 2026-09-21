# Multi-stage build for the Go data plane. One image, four binaries — the
# same source tree cmd/ulpf-collector, cmd/ulpf-processor, cmd/ulpfctl and
# cmd/ulpf-dlq-bridge already share (schema, vault, parse, normalize,
# enrich packages), so one build stage compiling all of them avoids
# separate dependency-download layers. docker-compose.yml's `command:`
# picks which binary each service actually runs.
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
# GOTOOLCHAIN=auto: go.mod may pin a toolchain newer than this base image's
# go1.23 (e.g. a version bump on the dev machine) — auto lets `go` fetch the
# exact required toolchain at BUILD time, which is fine (CLAUDE.md's
# air-gap rule is about the RUNNING system making zero outbound calls; a
# build-time fetch is the same category as pulling the base image itself).
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ulpf-collector ./cmd/ulpf-collector \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/ulpf-processor ./cmd/ulpf-processor \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/ulpfctl ./cmd/ulpfctl \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/ulpf-dlq-bridge ./cmd/ulpf-dlq-bridge

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ulpf-collector /out/ulpf-processor /out/ulpfctl /out/ulpf-dlq-bridge /app/
# packs/ (parser YAML), config/dictionaries/, enrichment/ are mounted as
# volumes in docker-compose.yml, not baked in — parsers are data, hot-
# swapped at runtime per CLAUDE.md, and rebuilding this image to publish a
# parser update would defeat that.
# No ENTRYPOINT/CMD: docker-compose.yml sets `command:` per service
# (/app/ulpf-collector, /app/ulpf-processor) so one image serves both.
