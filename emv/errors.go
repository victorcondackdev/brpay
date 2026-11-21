package emv

import (
	"errors"
	"strconv"
)

var (
	// ErrSyntax is wrapped by a *ParseError when the input is not a valid
	// TLV sequence (bad ID, bad length, truncated value).
	ErrSyntax = errors.New("emv: malformed TLV")

	// ErrStructure is wrapped by a *ParseError when the TLV is well formed
	// but the payload layout is invalid (e.g. field 00 not first, 63 not last).
	ErrStructure = errors.New("emv: invalid payload structure")

	// ErrCRC is wrapped by a *CRCError when the checksum in field 63 does not
	// match the payload.
	ErrCRC = errors.New("emv: CRC mismatch")

	// ErrEncode is returned by Encode when a field cannot be represented.
	ErrEncode = errors.New("emv: cannot encode field")
)

// ParseError describes where and why decoding a payload failed.
type ParseError struct {
	// Offset is the byte offset in the input where the problem was found.
	Offset int
	// ID is the field ID involved, as a dotted path for nested fields
	// (e.g. "62.05"). It is empty when the ID itself could not be read.
	ID string
	// Reason is a human-readable description of the problem.
	Reason string
	// Err is the error class: ErrSyntax or ErrStructure.
	Err error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	s := "emv: byte " + strconv.Itoa(e.Offset)
	if e.ID != "" {
		s += ", field " + e.ID
	}
	return s + ": " + e.Reason
}

// Unwrap returns the error class, so errors.Is(err, ErrSyntax) works.
func (e *ParseError) Unwrap() error { return e.Err }

// CRCError reports a checksum mismatch in field 63.
type CRCError struct {
	// Got is the CRC found in the payload, as written.
	Got string
	// Want is the CRC computed over the payload (4 uppercase hex digits).
	Want string
}

// Error implements the error interface.
func (e *CRCError) Error() string {
	return "emv: CRC mismatch: payload has " + strconv.Quote(e.Got) + ", computed " + e.Want
}

// Unwrap returns ErrCRC.
func (e *CRCError) Unwrap() error { return ErrCRC }
