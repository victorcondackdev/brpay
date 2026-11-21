package pix

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// EndToEndIDLength is the length of a Pix EndToEndId.
const EndToEndIDLength = 32

const (
	e2eTimeLayout  = "200601021504" // yyyyMMddHHmm
	e2eSequenceLen = 11
	e2eAlphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// EndToEndID is a parsed Pix EndToEndId: "E", the payer PSP's 8-digit ISPB,
// the creation time as yyyyMMddHHmm in UTC and an 11-character alphanumeric
// sequence, 32 characters in all.
type EndToEndID struct {
	ISPB     string    // 8-digit ISPB of the PSP that created the id
	Time     time.Time // creation time, UTC, minute precision
	Sequence string    // 11 alphanumeric characters
}

// String returns the 32-character EndToEndId.
func (e EndToEndID) String() string {
	return "E" + e.ISPB + e.Time.UTC().Format(e2eTimeLayout) + e.Sequence
}

// ParseEndToEndID parses and validates an EndToEndId. The timestamp must be
// a real calendar date and time. Errors wrap ErrInvalidEndToEndID.
func ParseEndToEndID(s string) (EndToEndID, error) {
	if len(s) != EndToEndIDLength {
		return EndToEndID{}, fmt.Errorf("%w: length %d, want %d", ErrInvalidEndToEndID, len(s), EndToEndIDLength)
	}
	if s[0] != 'E' {
		return EndToEndID{}, fmt.Errorf("%w: must start with \"E\"", ErrInvalidEndToEndID)
	}
	ispb, ts, seq := s[1:9], s[9:21], s[21:]
	if !isDigits(ispb) {
		return EndToEndID{}, fmt.Errorf("%w: ISPB %q must be 8 digits", ErrInvalidEndToEndID, ispb)
	}
	if !isDigits(ts) {
		return EndToEndID{}, fmt.Errorf("%w: timestamp %q must be 12 digits", ErrInvalidEndToEndID, ts)
	}
	t, err := time.Parse(e2eTimeLayout, ts)
	if err != nil {
		return EndToEndID{}, fmt.Errorf("%w: timestamp %q is not a valid date", ErrInvalidEndToEndID, ts)
	}
	for i := 0; i < len(seq); i++ {
		if !isAlnum(seq[i]) {
			return EndToEndID{}, fmt.Errorf("%w: sequence %q must be alphanumeric", ErrInvalidEndToEndID, seq)
		}
	}
	return EndToEndID{ISPB: ispb, Time: t, Sequence: seq}, nil
}

// NewEndToEndID creates an EndToEndId for the PSP with the given 8-digit
// ISPB at time t (converted to UTC, truncated to the minute). The sequence
// is drawn uniformly from [0-9A-Za-z] using rnd, or crypto/rand when rnd is
// nil.
func NewEndToEndID(ispb string, t time.Time, rnd io.Reader) (string, error) {
	if len(ispb) != 8 || !isDigits(ispb) {
		return "", fmt.Errorf("%w: ISPB %q must be 8 digits", ErrInvalidEndToEndID, ispb)
	}
	t = t.UTC()
	if y := t.Year(); y < 0 || y > 9999 {
		return "", fmt.Errorf("%w: year %d out of range", ErrInvalidEndToEndID, y)
	}
	if rnd == nil {
		rnd = rand.Reader
	}
	var out [EndToEndIDLength]byte
	out[0] = 'E'
	copy(out[1:], ispb)
	t.AppendFormat(out[9:9:21], e2eTimeLayout)

	// Rejection sampling keeps the distribution uniform: 248 = 4 * 62.
	// The attempt cap only matters for a broken reader that keeps
	// returning bytes >= 248.
	var buf [16]byte
	for n, attempts := 21, 0; n < EndToEndIDLength; attempts++ {
		if attempts == 64 {
			return "", fmt.Errorf("pix: random source yields no usable bytes")
		}
		if _, err := io.ReadFull(rnd, buf[:]); err != nil {
			return "", fmt.Errorf("pix: reading random bytes: %w", err)
		}
		for _, c := range buf {
			if c < 248 && n < EndToEndIDLength {
				out[n] = e2eAlphabet[c%62]
				n++
			}
		}
	}
	return string(out[:]), nil
}
