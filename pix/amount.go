package pix

import (
	"errors"
	"strconv"
)

// maxAmountLength is the maximum length of field 54 (transaction amount).
const maxAmountLength = 13

// MaxCents is the largest amount, in cents, that fits in field 54
// ("9999999999.99").
const MaxCents = 999_999_999_999

var (
	errAmountSyntax   = errors.New("want digits with an optional dot and up to 2 decimals, e.g. 10.50")
	errAmountTooLong  = errors.New("longer than 13 characters")
	errAmountComma    = errors.New("uses a comma; the decimal separator must be a dot")
	errAmountDecimals = errors.New("more than 2 decimal places")
)

// parseAmount converts the value of field 54 to cents. It accepts
// "10", "10.5" and "10.50"; leading zeros are accepted (Lint warns).
func parseAmount(s string) (int64, error) {
	if len(s) == 0 || len(s) > maxAmountLength {
		if len(s) > maxAmountLength {
			return 0, errAmountTooLong
		}
		return 0, errAmountSyntax
	}
	var whole, frac int64
	dot, decimals := false, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if dot {
				decimals++
				if decimals > 2 {
					return 0, errAmountDecimals
				}
				frac = frac*10 + int64(c-'0')
			} else {
				whole = whole*10 + int64(c-'0')
			}
		case c == ',':
			return 0, errAmountComma
		case c == '.' && !dot && i > 0:
			dot = true
		default:
			return 0, errAmountSyntax
		}
	}
	if dot && decimals == 0 {
		return 0, errAmountSyntax
	}
	if decimals == 1 {
		frac *= 10
	}
	return whole*100 + frac, nil
}

// formatAmount renders cents as field 54 expects: "10.50", "0.01".
func formatAmount(cents int64) string {
	var buf [24]byte
	b := strconv.AppendInt(buf[:0], cents/100, 10)
	b = append(b, '.', byte('0'+cents%100/10), byte('0'+cents%10))
	return string(b)
}
