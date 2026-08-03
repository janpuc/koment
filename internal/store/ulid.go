package store

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	ulidLength     = 26
	bitsPerChar    = 5
	timestampBytes = 6
	entropyBytes   = 10

	paddingBits = ulidLength*bitsPerChar - 8*(timestampBytes+entropyBytes)
)

// NewID returns a ULID: a 48-bit millisecond timestamp followed by 80 random
// bits, rendered as 26 Crockford base32 characters that sort by creation time.
func NewID(at time.Time) (string, error) {
	milliseconds := at.UnixMilli()
	if milliseconds < 0 || milliseconds>>(8*timestampBytes) != 0 {
		return "", fmt.Errorf("timestamp %s does not fit in a ULID", at)
	}

	var id [timestampBytes + entropyBytes]byte
	var wide [8]byte
	binary.BigEndian.PutUint64(wide[:], uint64(milliseconds))
	copy(id[:timestampBytes], wide[8-timestampBytes:])

	if _, err := rand.Read(id[timestampBytes:]); err != nil {
		return "", fmt.Errorf("reading ULID entropy: %w", err)
	}
	return encodeCrockford(id), nil
}

func ValidID(id string) bool {
	if len(id) != ulidLength || id[0] > '7' {
		return false
	}
	for _, character := range id {
		if !strings.ContainsRune(crockfordAlphabet, character) {
			return false
		}
	}
	return true
}

func encodeCrockford(id [timestampBytes + entropyBytes]byte) string {
	out := make([]byte, ulidLength)
	for i := range out {
		out[i] = crockfordAlphabet[fiveBitsAt(id, i*bitsPerChar)]
	}
	return string(out)
}

func fiveBitsAt(id [timestampBytes + entropyBytes]byte, offset int) byte {
	var value byte
	for i := range bitsPerChar {
		value <<= 1
		if bitSet(id, offset+i-paddingBits) {
			value |= 1
		}
	}
	return value
}

func bitSet(id [timestampBytes + entropyBytes]byte, n int) bool {
	if n < 0 {
		return false
	}
	return id[n/8]&(1<<(7-n%8)) != 0
}
