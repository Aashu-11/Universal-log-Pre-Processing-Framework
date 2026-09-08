// Package uesschema embeds the canonical UES 1.0 JSON Schema so it ships
// inside every compiled binary (ulpf-processor, ulpfctl) instead of being
// read from a file path that may not exist at the deployment target. This
// file lives next to ues-1.0.json specifically so go:embed can reach it —
// embed directives cannot cross into a parent directory, so the embedding
// package has to live where the data lives. It is named "uesschema", not
// "schema", so it doesn't collide with the internal/schema package name
// when both are imported together.
package uesschema

import _ "embed"

//go:embed ues-1.0.json
var UES10JSON []byte
