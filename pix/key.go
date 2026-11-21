package pix

import (
	"fmt"
	"strings"

	"github.com/victorcondackdev/brpay/document"
)

// KeyType is the type of a Pix key ("chave Pix") as registered in the DICT.
type KeyType int

// Pix key types.
const (
	KeyUnknown KeyType = iota
	KeyCPF             // 11-digit CPF
	KeyCNPJ            // 14-character CNPJ, numeric or alphanumeric
	KeyEmail           // e-mail address, lowercase, at most 77 characters
	KeyPhone           // Brazilian mobile number in E.164 format, e.g. +5521987654321
	KeyEVP             // random key: lowercase UUID with hyphens
)

// String returns the DICT name of the key type: CPF, CNPJ, EMAIL, PHONE or
// EVP, or UNKNOWN.
func (k KeyType) String() string {
	switch k {
	case KeyCPF:
		return "CPF"
	case KeyCNPJ:
		return "CNPJ"
	case KeyEmail:
		return "EMAIL"
	case KeyPhone:
		return "PHONE"
	case KeyEVP:
		return "EVP"
	default:
		return "UNKNOWN"
	}
}

// MaxEmailKeyLength is the maximum length of an e-mail Pix key.
const MaxEmailKeyLength = 77

// DetectKey returns the type of a Pix key. See NormalizeKey for the rules.
func DetectKey(s string) (KeyType, error) {
	_, t, err := NormalizeKey(s)
	return t, err
}

// NormalizeKey validates a Pix key and returns it in the canonical form used
// by the DICT and inside BR Codes, along with its type. Surrounding
// whitespace is ignored. The type is decided by shape, in this order:
//
//   - contains "@": e-mail, lowercased, at most 77 characters, ASCII only,
//     with a dotted domain.
//   - starts with "+": phone. Spaces, hyphens, dots and parentheses are
//     removed. The number must be +55, a 2-digit area code (DDD) without
//     zeros and 8 digits, or 9 digits starting with 9. National numbers
//     without "+55" are not accepted, because an 11-digit string is
//     indistinguishable from a CPF.
//   - UUID shape (36 characters, hyphens at positions 8, 13, 18, 23): EVP,
//     lowercased. The UUID version is not checked, as the DICT does not
//     constrain it.
//   - otherwise, after removing ".", "-", "/" and spaces: 11 characters is a
//     CPF and 14 is a CNPJ (alphanumeric allowed, uppercased); both must have
//     valid check digits.
//
// Errors wrap ErrInvalidKey.
func NormalizeKey(s string) (string, KeyType, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", KeyUnknown, fmt.Errorf("%w: empty", ErrInvalidKey)
	case strings.IndexByte(s, '@') >= 0:
		k, err := normalizeEmail(s)
		return k, typeIf(err, KeyEmail), err
	case s[0] == '+':
		k, err := normalizePhone(s)
		return k, typeIf(err, KeyPhone), err
	case isUUIDShape(s):
		k, err := normalizeEVP(s)
		return k, typeIf(err, KeyEVP), err
	}
	c := document.Clean(s)
	switch len(c) {
	case 11:
		if err := document.ValidateCPF(c); err != nil {
			return "", KeyUnknown, fmt.Errorf("%w: CPF: %w (phone keys must start with +55)", ErrInvalidKey, err)
		}
		return c, KeyCPF, nil
	case 14:
		if err := document.ValidateCNPJ(c); err != nil {
			return "", KeyUnknown, fmt.Errorf("%w: CNPJ: %w", ErrInvalidKey, err)
		}
		return c, KeyCNPJ, nil
	}
	return "", KeyUnknown, fmt.Errorf("%w: not a CPF, CNPJ, e-mail, phone (+55...) or EVP", ErrInvalidKey)
}

func typeIf(err error, t KeyType) KeyType {
	if err != nil {
		return KeyUnknown
	}
	return t
}

func normalizeEmail(s string) (string, error) {
	if len(s) > MaxEmailKeyLength {
		return "", fmt.Errorf("%w: e-mail longer than %d characters", ErrInvalidKey, MaxEmailKeyLength)
	}
	e := []byte(s)
	for i, c := range e {
		if c >= 'A' && c <= 'Z' {
			e[i] = c + 'a' - 'A'
		}
	}
	at := strings.IndexByte(s, '@')
	if at != strings.LastIndexByte(s, '@') {
		return "", fmt.Errorf("%w: e-mail has more than one @", ErrInvalidKey)
	}
	local, domain := e[:at], e[at+1:]
	if len(local) == 0 || len(local) > 64 {
		return "", fmt.Errorf("%w: e-mail local part must have 1 to 64 characters", ErrInvalidKey)
	}
	if local[0] == '.' || local[len(local)-1] == '.' || strings.Contains(string(local), "..") {
		return "", fmt.Errorf("%w: misplaced dot in e-mail local part", ErrInvalidKey)
	}
	for _, c := range local {
		if !isAlnumLower(c) && strings.IndexByte(".!#$%&'*+/=?^_`{|}~-", c) < 0 {
			return "", fmt.Errorf("%w: invalid character %q in e-mail", ErrInvalidKey, rune(c))
		}
	}
	labels := strings.Split(string(domain), ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: e-mail domain must contain a dot", ErrInvalidKey)
	}
	for _, l := range labels {
		if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return "", fmt.Errorf("%w: invalid e-mail domain", ErrInvalidKey)
		}
		for i := 0; i < len(l); i++ {
			if !isAlnumLower(l[i]) && l[i] != '-' {
				return "", fmt.Errorf("%w: invalid character %q in e-mail domain", ErrInvalidKey, rune(l[i]))
			}
		}
	}
	return string(e), nil
}

func normalizePhone(s string) (string, error) {
	var buf [16]byte
	n := 0
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if n == len(buf) {
				return "", fmt.Errorf("%w: phone number too long", ErrInvalidKey)
			}
			buf[n] = c
			n++
		case c == ' ' || c == '-' || c == '.' || c == '(' || c == ')':
		default:
			return "", fmt.Errorf("%w: invalid character %q in phone number", ErrInvalidKey, c)
		}
	}
	d := buf[:n]
	if n < 2 || d[0] != '5' || d[1] != '5' {
		return "", fmt.Errorf("%w: phone keys must be Brazilian numbers (+55)", ErrInvalidKey)
	}
	nat := d[2:]
	switch {
	case len(nat) != 10 && len(nat) != 11:
		return "", fmt.Errorf("%w: phone number must have a 2-digit area code and 8 or 9 digits", ErrInvalidKey)
	case nat[0] == '0' || nat[1] == '0':
		return "", fmt.Errorf("%w: invalid area code (DDD) %s", ErrInvalidKey, nat[:2])
	case len(nat) == 11 && nat[2] != '9':
		return "", fmt.Errorf("%w: 9-digit phone numbers must start with 9", ErrInvalidKey)
	}
	return "+" + string(d), nil
}

func isUUIDShape(s string) bool {
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

func normalizeEVP(s string) (string, error) {
	b := []byte(s)
	for i, c := range b {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
			b[i] = c + 'a' - 'A'
		default:
			return "", fmt.Errorf("%w: EVP must be a hexadecimal UUID", ErrInvalidKey)
		}
	}
	return string(b), nil
}

func isAlnumLower(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
