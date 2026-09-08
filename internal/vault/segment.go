package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

// Segment wire format (all inside the zstd-compressed object body):
//
//	[header]  magic "ULPFVLT1" (8) | segment_id_len (uvarint) | segment_id bytes
//	[frames]  repeated: len (uvarint) | payload (len bytes)      <- RawRef.Offset points at "payload" here
//	[index]   repeated, one per frame in order: offset (uvarint) | length (uvarint) | sha256 (32 bytes)
//	[trailer] merkle_root (32) | prev_root (32) | index_offset (8, BE uint64) | event_count (8, BE uint64) | magic "ULPFEND1" (8)
//
// The trailer is fixed-size (88 bytes) and sits at the very end, so a reader
// can locate the index block and both roots without scanning from the
// start. RawRef.Offset/Length always point directly at frame payload bytes,
// so Read() never needs the index — it's there for VerifyRange/ProveEvent,
// which need the full ordered leaf list.

var (
	segmentMagicHeader = [8]byte{'U', 'L', 'P', 'F', 'V', 'L', 'T', '1'}
	segmentMagicFooter = [8]byte{'U', 'L', 'P', 'F', 'E', 'N', 'D', '1'}
)

const trailerSize = 32 + 32 + 8 + 8 + 8

type indexEntry struct {
	offset uint64
	length uint64
	sha    [32]byte
}

// newSegmentBuf starts a new segment buffer and writes its header. Frames
// are appended incrementally via appendFrame as events arrive, so a
// segment's in-progress byte size is always known for the seal-at-64MB
// check without re-serializing anything.
func newSegmentBuf(segmentID string) *bytes.Buffer {
	var buf bytes.Buffer
	buf.Write(segmentMagicHeader[:])
	idLen := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(idLen, uint64(len(segmentID)))
	buf.Write(idLen[:n])
	buf.WriteString(segmentID)
	return &buf
}

// appendFrame writes one length-prefixed payload to buf and returns the
// absolute offset of the payload bytes (what RawRef.Offset will point at)
// plus its SHA-256.
func appendFrame(buf *bytes.Buffer, payload []byte) (offset uint64, sha [32]byte) {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	ln := binary.PutUvarint(lenBuf, uint64(len(payload)))
	buf.Write(lenBuf[:ln])

	offset = uint64(buf.Len())
	buf.Write(payload)
	sha = sha256.Sum256(payload)
	return offset, sha
}

// sealSegmentBuf appends the index block and trailer to buf (which must
// already contain the header and every frame) and returns the segment's
// Merkle root. buf is ready to compress once this returns.
func sealSegmentBuf(buf *bytes.Buffer, entries []indexEntry, leaves [][32]byte, prevRoot [32]byte) [32]byte {
	indexOffset := uint64(buf.Len())
	for _, e := range entries {
		tmp := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(tmp, e.offset)
		buf.Write(tmp[:n])
		n = binary.PutUvarint(tmp, e.length)
		buf.Write(tmp[:n])
		buf.Write(e.sha[:])
	}

	root := merkleRoot(leaves)

	buf.Write(root[:])
	buf.Write(prevRoot[:])

	var fixed [24]byte
	binary.BigEndian.PutUint64(fixed[0:8], indexOffset)
	binary.BigEndian.PutUint64(fixed[8:16], uint64(len(entries)))
	copy(fixed[16:24], segmentMagicFooter[:])
	buf.Write(fixed[:])

	return root
}

type decodedSegment struct {
	segmentID string
	merkle    [32]byte
	prevRoot  [32]byte
	entries   []indexEntry
	raw       []byte // the full decompressed segment buffer
}

// decodeSegment parses a decompressed segment buffer back into its header,
// trailer and index. It does not re-verify hashes — callers that need
// integrity verification (VerifyRange) do that explicitly against raw.
func decodeSegment(data []byte) (*decodedSegment, error) {
	if len(data) < 8+trailerSize {
		return nil, fmt.Errorf("vault: segment too short (%d bytes)", len(data))
	}
	if !bytes.Equal(data[0:8], segmentMagicHeader[:]) {
		return nil, fmt.Errorf("vault: bad segment header magic")
	}

	trailer := data[len(data)-trailerSize:]
	if !bytes.Equal(trailer[80:88], segmentMagicFooter[:]) {
		return nil, fmt.Errorf("vault: bad segment footer magic")
	}
	// Trailer layout: [0:32]=root [32:64]=prev [64:72]=index_offset [72:80]=count [80:88]=magic
	var merkle, prevRoot [32]byte
	copy(merkle[:], trailer[0:32])
	copy(prevRoot[:], trailer[32:64])
	indexOffset := binary.BigEndian.Uint64(trailer[64:72])
	eventCount := binary.BigEndian.Uint64(trailer[72:80])

	_, id, err := readHeaderID(data)
	if err != nil {
		return nil, err
	}

	indexBlock := data[indexOffset : uint64(len(data))-trailerSize]
	entries := make([]indexEntry, 0, eventCount)
	r := bytes.NewReader(indexBlock)
	for i := uint64(0); i < eventCount; i++ {
		off, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("vault: read index offset %d: %w", i, err)
		}
		length, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, fmt.Errorf("vault: read index length %d: %w", i, err)
		}
		var sha [32]byte
		if _, err := io.ReadFull(r, sha[:]); err != nil {
			return nil, fmt.Errorf("vault: read index sha %d: %w", i, err)
		}
		entries = append(entries, indexEntry{offset: off, length: length, sha: sha})
	}

	return &decodedSegment{
		segmentID: id,
		merkle:    merkle,
		prevRoot:  prevRoot,
		entries:   entries,
		raw:       data,
	}, nil
}

func readHeaderID(data []byte) (int, string, error) {
	r := bytes.NewReader(data[8:])
	idLen, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, "", fmt.Errorf("vault: read segment id length: %w", err)
	}
	idBytes := make([]byte, idLen)
	if _, err := io.ReadFull(r, idBytes); err != nil {
		return 0, "", fmt.Errorf("vault: read segment id: %w", err)
	}
	consumed := len(data) - r.Len()
	return consumed, string(idBytes), nil
}
