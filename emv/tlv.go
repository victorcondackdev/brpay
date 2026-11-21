package emv

import (
	"strconv"
	"strings"
)

// MaxValueLength is the largest value a single data object can hold, as the
// length is written with two decimal digits.
const MaxValueLength = 99

// Field is a single EMV data object.
//
// Value always holds the raw value. For template IDs (see IsTemplateID),
// Parse also decodes the value into Children; Encode prefers Children over
// Value when Children is non-empty.
type Field struct {
	ID       string
	Value    string
	Children []Field
}

// IsTemplateID reports whether a top-level ID denotes a template, i.e. a
// field whose value is itself a TLV list: 26–51 (merchant account
// information), 62 (additional data), 64 (merchant information, alternate
// language) and 80–99 (unreserved templates).
func IsTemplateID(id string) bool {
	n, ok := atoi2(id)
	if !ok {
		return false
	}
	return (n >= 26 && n <= 51) || n == 62 || n == 64 || n >= 80
}

// Find returns the first field with the given ID.
func Find(fields []Field, id string) (Field, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, true
		}
	}
	return Field{}, false
}

// Parse decodes a payload into its top-level fields, in order, expanding
// template fields (see IsTemplateID) one level deep. It checks TLV syntax
// only; use Validate for payload structure and VerifyCRC for integrity.
// Errors are of type *ParseError with offsets relative to payload.
func Parse(payload string) ([]Field, error) {
	return parse(payload, 0, "", true)
}

// ParseTemplate decodes the value of a template field into a flat list of
// fields. Offsets in errors are relative to value.
func ParseTemplate(value string) ([]Field, error) {
	return parse(value, 0, "", false)
}

func parse(s string, base int, parent string, expand bool) ([]Field, error) {
	// Each field takes at least 5 bytes; a rough capacity avoids regrowth.
	fields := make([]Field, 0, len(s)/12+1)
	for i := 0; i < len(s); {
		if rem := len(s) - i; rem < 4 {
			return nil, &ParseError{Offset: base + i, ID: parent,
				Reason: "truncated field header: need 4 bytes, have " + strconv.Itoa(rem), Err: ErrSyntax}
		}
		id := s[i : i+2]
		if _, ok := atoi2(id); !ok {
			return nil, &ParseError{Offset: base + i, ID: parent,
				Reason: "invalid field ID " + strconv.Quote(id) + ": want 2 digits", Err: ErrSyntax}
		}
		n, ok := atoi2(s[i+2 : i+4])
		if !ok {
			return nil, &ParseError{Offset: base + i + 2, ID: joinID(parent, id),
				Reason: "invalid length " + strconv.Quote(s[i+2:i+4]) + ": want 2 digits", Err: ErrSyntax}
		}
		if n == 0 {
			return nil, &ParseError{Offset: base + i + 2, ID: joinID(parent, id), Reason: "zero length", Err: ErrSyntax}
		}
		start := i + 4
		if rem := len(s) - start; n > rem {
			return nil, &ParseError{Offset: base + i, ID: joinID(parent, id),
				Reason: "length " + strconv.Itoa(n) + " exceeds remaining " + strconv.Itoa(rem), Err: ErrSyntax}
		}
		f := Field{ID: id, Value: s[start : start+n]}
		if expand && IsTemplateID(id) {
			children, err := parse(f.Value, base+start, id, false)
			if err != nil {
				return nil, err
			}
			f.Children = children
		}
		fields = append(fields, f)
		i = start + n
	}
	return fields, nil
}

// Encode serializes fields in order, computing every length. Fields with
// Children are encoded recursively. It does not add a CRC; see AppendCRC.
// It returns an error wrapping ErrEncode if an ID is not two digits or a
// value is empty or longer than MaxValueLength.
func Encode(fields []Field) (string, error) {
	var b strings.Builder
	if err := encodeTo(&b, fields, ""); err != nil {
		return "", err
	}
	return b.String(), nil
}

func encodeTo(b *strings.Builder, fields []Field, prefix string) error {
	for _, f := range fields {
		if _, ok := atoi2(f.ID); !ok {
			return &encodeError{id: prefix + f.ID, reason: "ID must be 2 digits"}
		}
		v := f.Value
		if len(f.Children) > 0 {
			var cb strings.Builder
			if err := encodeTo(&cb, f.Children, prefix+f.ID+"."); err != nil {
				return err
			}
			v = cb.String()
		}
		switch {
		case len(v) == 0:
			return &encodeError{id: prefix + f.ID, reason: "empty value"}
		case len(v) > MaxValueLength:
			return &encodeError{id: prefix + f.ID, reason: "value is " + strconv.Itoa(len(v)) + " bytes, max 99"}
		}
		b.WriteString(f.ID)
		b.WriteByte(byte('0' + len(v)/10))
		b.WriteByte(byte('0' + len(v)%10))
		b.WriteString(v)
	}
	return nil
}

type encodeError struct{ id, reason string }

func (e *encodeError) Error() string { return ErrEncode.Error() + " " + e.id + ": " + e.reason }
func (e *encodeError) Unwrap() error { return ErrEncode }

// Validate checks the structure of a parsed payload: field 00 (payload
// format indicator) must come first with value "01", field 63 (CRC) must
// come last with a 4-character value, and no top-level ID may repeat.
// Errors are of type *ParseError wrapping ErrStructure, with offsets as they
// would be in the encoded payload.
func Validate(fields []Field) error {
	if len(fields) == 0 {
		return &ParseError{Reason: "empty payload", Err: ErrStructure}
	}
	if f := fields[0]; f.ID != "00" {
		return &ParseError{ID: f.ID, Reason: "payload format indicator (00) must be the first field", Err: ErrStructure}
	} else if f.Value != "01" {
		return &ParseError{ID: "00", Reason: "payload format indicator must be \"01\", got " + strconv.Quote(f.Value), Err: ErrStructure}
	}
	var seen [100]bool
	off := 0
	for i, f := range fields {
		n, ok := atoi2(f.ID)
		if !ok {
			return &ParseError{Offset: off, ID: f.ID, Reason: "invalid field ID", Err: ErrStructure}
		}
		if seen[n] {
			return &ParseError{Offset: off, ID: f.ID, Reason: "duplicate field", Err: ErrStructure}
		}
		seen[n] = true
		last := i == len(fields)-1
		switch {
		case last && f.ID != "63":
			return &ParseError{Offset: off, ID: f.ID, Reason: "CRC (63) must be the last field", Err: ErrStructure}
		case f.ID == "63" && !last:
			return &ParseError{Offset: off, ID: "63", Reason: "CRC (63) must be the last field", Err: ErrStructure}
		case f.ID == "63" && len(f.Value) != 4:
			return &ParseError{Offset: off, ID: "63", Reason: "CRC must be 4 characters, got " + strconv.Itoa(len(f.Value)), Err: ErrStructure}
		}
		off += 4 + valueLen(f)
	}
	return nil
}

// valueLen is the encoded length of a field's value.
func valueLen(f Field) int {
	if len(f.Children) == 0 {
		return len(f.Value)
	}
	n := 0
	for _, c := range f.Children {
		n += 4 + valueLen(c)
	}
	return n
}

// joinID returns the dotted path of a nested field ID.
func joinID(parent, id string) string {
	if parent == "" {
		return id
	}
	return parent + "." + id
}

// atoi2 parses exactly two ASCII digits.
func atoi2(s string) (int, bool) {
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}
