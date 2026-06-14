// Package domain holds the core ROTASAVINGS entities and the rules that are
// mirrored from the on-chain truth layer. Nothing here is the source of truth;
// the smart contracts are. These types are the shapes the orchestration layer
// and the cache (Postgres) project on-chain state into.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Money is an integer amount in the smallest currency unit (e.g. kobo/cents).
// We never use floats for money.
type Money int64

func (m Money) String() string { return fmt.Sprintf("%d", int64(m)) }

// NewID returns a random 128-bit hex identifier used for off-chain row keys.
// On-chain identifiers (addresses, tx hashes) come from the chain layer.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is unrecoverable for our purposes.
		panic("domain: cannot read randomness: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// Hash deterministically commits to an ordered list of fields. It is the
// off-chain mirror of the commitments the contracts compute, so the same
// inputs must always produce the same digest. Fields are separated by a unit
// separator byte to avoid ambiguity (e.g. "ab"+"c" vs "a"+"bc").
func Hash(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0x1f})
		}
		h.Write([]byte(p))
	}
	return "0x" + hex.EncodeToString(h.Sum(nil))
}
