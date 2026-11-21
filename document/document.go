// Package document validates and formats Brazilian taxpayer identifiers:
// CPF (individuals) and CNPJ (companies), including the alphanumeric CNPJ
// format that the Receita Federal starts issuing in July 2026.
//
// Alphanumeric CNPJ: the first 12 characters may be digits or uppercase
// letters A–Z; the last two (check digits) are always numeric. The check
// digits are computed with the classic modulo-11 weights, using the value
// (ASCII code − 48) for every character, which keeps purely numeric CNPJs
// backward compatible.
package document

import (
	"errors"
	"strings"
)

// Kind identifies the type of a document.
type Kind int

const (
	Unknown Kind = iota
	CPF
	CNPJ
)

func (k Kind) String() string {
	switch k {
	case CPF:
		return "CPF"
	case CNPJ:
		return "CNPJ"
	default:
		return "unknown"
	}
}

var (
	ErrLength     = errors.New("document: invalid length")
	ErrCharacters = errors.New("document: invalid characters")
	ErrRepeated   = errors.New("document: all digits repeated")
	ErrCheckDigit = errors.New("document: check digit mismatch")
)

// Clean removes the usual punctuation (".", "-", "/", spaces) and uppercases
// letters. It does not validate.
func Clean(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '/' || r == ' ':
			// separator, drop
		default:
			b.WriteRune(r) // keep so validation can reject it
		}
	}
	return b.String()
}

// Detect returns CPF or CNPJ when s (after Clean) is a valid document.
func Detect(s string) Kind {
	c := Clean(s)
	if ValidateCPF(c) == nil {
		return CPF
	}
	if ValidateCNPJ(c) == nil {
		return CNPJ
	}
	return Unknown
}

// ValidateCPF validates an 11-digit CPF (punctuation allowed).
func ValidateCPF(s string) error {
	c := Clean(s)
	if len(c) != 11 {
		return ErrLength
	}
	for i := 0; i < 11; i++ {
		if c[i] < '0' || c[i] > '9' {
			return ErrCharacters
		}
	}
	if allSame(c) {
		return ErrRepeated
	}
	d1 := cpfDigit(c[:9], 10)
	d2 := cpfDigit(c[:9]+string(rune('0'+d1)), 11)
	if int(c[9]-'0') != d1 || int(c[10]-'0') != d2 {
		return ErrCheckDigit
	}
	return nil
}

func cpfDigit(base string, weight int) int {
	sum := 0
	for i := 0; i < len(base); i++ {
		sum += int(base[i]-'0') * (weight - i)
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// ValidateCNPJ validates a numeric or alphanumeric CNPJ (punctuation allowed).
func ValidateCNPJ(s string) error {
	c := Clean(s)
	if len(c) != 14 {
		return ErrLength
	}
	for i := 0; i < 12; i++ {
		ch := c[i]
		if !(ch >= '0' && ch <= '9') && !(ch >= 'A' && ch <= 'Z') {
			return ErrCharacters
		}
	}
	if c[12] < '0' || c[12] > '9' || c[13] < '0' || c[13] > '9' {
		return ErrCharacters
	}
	if allSame(c) {
		return ErrRepeated
	}
	d1, d2 := CNPJCheckDigits(c[:12])
	if int(c[12]-'0') != d1 || int(c[13]-'0') != d2 {
		return ErrCheckDigit
	}
	return nil
}

// CNPJCheckDigits computes the two check digits for a 12-character CNPJ base
// (numeric or alphanumeric, already cleaned and uppercased).
func CNPJCheckDigits(base string) (int, int) {
	d1 := cnpjDigit(base)
	d2 := cnpjDigit(base + string(rune('0'+d1)))
	return d1, d2
}

func cnpjDigit(base string) int {
	sum, w := 0, 2
	for i := len(base) - 1; i >= 0; i-- {
		sum += int(base[i]-'0') * w
		w++
		if w > 9 {
			w = 2
		}
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

// IsAlphanumericCNPJ reports whether a valid CNPJ uses the new
// alphanumeric format (contains at least one letter).
func IsAlphanumericCNPJ(s string) bool {
	c := Clean(s)
	if ValidateCNPJ(c) != nil {
		return false
	}
	for i := 0; i < 12; i++ {
		if c[i] >= 'A' && c[i] <= 'Z' {
			return true
		}
	}
	return false
}

// Format returns the canonical punctuated form (000.000.000-00 or
// AA.AAA.AAA/AAAA-00). Invalid input is returned cleaned but unformatted.
func Format(s string) string {
	c := Clean(s)
	switch {
	case ValidateCPF(c) == nil:
		return c[0:3] + "." + c[3:6] + "." + c[6:9] + "-" + c[9:11]
	case ValidateCNPJ(c) == nil:
		return c[0:2] + "." + c[2:5] + "." + c[5:8] + "/" + c[8:12] + "-" + c[12:14]
	default:
		return c
	}
}

// Mask hides the middle of a document for logs, following the LGPD-friendly
// pattern used by the Pix DICT (***.456.789-**).
func Mask(s string) string {
	c := Clean(s)
	switch {
	case ValidateCPF(c) == nil:
		return "***." + c[3:6] + "." + c[6:9] + "-**"
	case ValidateCNPJ(c) == nil:
		return c[0:2] + ".***.***/" + c[8:12] + "-**"
	default:
		return strings.Repeat("*", len(c))
	}
}

func allSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
