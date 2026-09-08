package vault

import "crypto/sha256"

// merkleRoot builds a binary Merkle tree over leaves (in order) and returns
// the root hash. An odd node at any level is promoted by duplicating it
// (the standard Bitcoin-style construction) rather than left unhashed, so
// tree shape is a pure function of leaf count. Empty input returns the
// SHA-256 of the empty string, matching common "empty segment" tooling
// (never actually reached in practice — a segment seals with >=1 event).
func merkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256(nil)
	}
	level := make([][32]byte, len(leaves))
	copy(level, leaves)

	for len(level) > 1 {
		next := make([][32]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				next = append(next, hashPair(level[i], level[i+1]))
			} else {
				next = append(next, hashPair(level[i], level[i]))
			}
		}
		level = next
	}
	return level[0]
}

func hashPair(a, b [32]byte) [32]byte {
	h := sha256.New()
	h.Write(a[:])
	h.Write(b[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// MerkleProofStep is one sibling hash on the path from a leaf to the root.
type MerkleProofStep struct {
	Sibling   [32]byte
	SiblingOn string // "left" or "right" — which side Sibling sits relative to the running hash.
}

// MerkleProof is an inclusion proof: hashing Leaf up through Path in order
// must reproduce Root.
type MerkleProof struct {
	Leaf  [32]byte
	Root  [32]byte
	Path  []MerkleProofStep
	Index int
}

// buildMerkleProof returns the inclusion path for leaves[index] up to the
// tree's root, using the same pairing/duplication rule as merkleRoot.
func buildMerkleProof(leaves [][32]byte, index int) MerkleProof {
	proof := MerkleProof{Leaf: leaves[index], Index: index}

	level := make([][32]byte, len(leaves))
	copy(level, leaves)
	idx := index

	for len(level) > 1 {
		next := make([][32]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := level[i]
			if i+1 < len(level) {
				right = level[i+1]
			}
			if i == idx || i+1 == idx {
				if idx == i {
					proof.Path = append(proof.Path, MerkleProofStep{Sibling: right, SiblingOn: "right"})
				} else {
					proof.Path = append(proof.Path, MerkleProofStep{Sibling: left, SiblingOn: "left"})
				}
				idx = len(next)
			}
			next = append(next, hashPair(left, right))
		}
		level = next
	}
	proof.Root = level[0]
	return proof
}

// Verify recomputes the root from Leaf and Path and reports whether it
// matches Root.
func (p MerkleProof) Verify() bool {
	cur := p.Leaf
	for _, step := range p.Path {
		if step.SiblingOn == "left" {
			cur = hashPair(step.Sibling, cur)
		} else {
			cur = hashPair(cur, step.Sibling)
		}
	}
	return cur == p.Root
}
