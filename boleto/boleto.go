package boleto

import (
	"fmt"
	"unicode/utf8"
)

// Code lengths, in digits, of the supported forms.
const (
	BarcodeLen          = 44 // barcode of either family
	BancarioLineLen     = 47 // bancário linha digitável
	ArrecadacaoLineLen  = 48 // arrecadação linha digitável
	maxInputDigits      = ArrecadacaoLineLen
	arrecadacaoProduct  = '8'
	bancarioCurrencyBRL = 9
)

// Type identifies the boleto family.
type Type int

// Boleto families.
const (
	TypeUnknown     Type = iota
	TypeBancario         // bank slip ("boleto de cobrança")
	TypeArrecadacao      // utility bill, tax or fine ("convênio")
)

// String returns the Portuguese name of the family.
func (t Type) String() string {
	switch t {
	case TypeBancario:
		return "bancário"
	case TypeArrecadacao:
		return "arrecadação"
	default:
		return "unknown"
	}
}

// Boleto is a parsed and fully validated boleto. Exactly one of Bancario and
// Arrecadacao is non-nil, according to Type.
type Boleto struct {
	// Type is the boleto family.
	Type Type
	// Barcode is the 44-digit barcode.
	Barcode string
	// DigitableLine is the linha digitável as bare digits (47 or 48).
	DigitableLine string
	// AmountCents is the amount field in cents. For arrecadação codes whose
	// value is a reference (see [Arrecadacao.ValueIsReference]) it holds the
	// raw reference value instead of money.
	AmountCents int64
	// Bancario holds the bank-slip specific fields.
	Bancario *Bancario
	// Arrecadacao holds the utility-bill specific fields.
	Arrecadacao *Arrecadacao
}

// Formatted returns the linha digitável in its printed form:
// "AAAAA.AAAAA BBBBB.BBBBBB CCCCC.CCCCCC D EEEEEEEEEEEEEE" for bancário and
// "XXXXXXXXXXX-D XXXXXXXXXXX-D XXXXXXXXXXX-D XXXXXXXXXXX-D" for arrecadação.
func (b *Boleto) Formatted() string {
	return formatLine(b.DigitableLine)
}

// String returns [Boleto.Formatted].
func (b *Boleto) String() string { return b.Formatted() }

// Parse parses a barcode (44 digits) or a linha digitável (47 digits for
// bancário, 48 for arrecadação). Spaces, tabs, newlines, dots and hyphens
// are ignored. Every check digit is verified; a mismatch is returned as a
// [*CheckDigitError].
func Parse(input string) (*Boleto, error) {
	d, err := clean(input)
	if err != nil {
		return nil, err
	}
	switch len(d) {
	case BarcodeLen:
		return parseBarcode(d)
	case BancarioLineLen:
		return parseBancarioLine(d)
	case ArrecadacaoLineLen:
		return parseArrecadacaoLine(d)
	default:
		return nil, lengthError(len(d))
	}
}

// Validate reports whether input is a valid boleto barcode or linha
// digitável. It returns nil or the same error [Parse] would.
func Validate(input string) error {
	_, err := Parse(input)
	return err
}

// BarcodeToLine converts a 44-digit barcode of either family into its linha
// digitável (bare digits), validating it first.
func BarcodeToLine(code string) (string, error) {
	d, err := clean(code)
	if err != nil {
		return "", err
	}
	if len(d) != BarcodeLen {
		return "", fmt.Errorf("%w: got %d digits, want %d", ErrLength, len(d), BarcodeLen)
	}
	b, err := parseBarcode(d)
	if err != nil {
		return "", err
	}
	return b.DigitableLine, nil
}

// LineToBarcode converts a linha digitável of either family (47 or 48
// digits, punctuation allowed) into its 44-digit barcode, validating it
// first.
func LineToBarcode(line string) (string, error) {
	d, err := clean(line)
	if err != nil {
		return "", err
	}
	var b *Boleto
	switch len(d) {
	case BancarioLineLen:
		b, err = parseBancarioLine(d)
	case ArrecadacaoLineLen:
		b, err = parseArrecadacaoLine(d)
	default:
		return "", fmt.Errorf("%w: got %d digits, want %d or %d", ErrLength, len(d), BancarioLineLen, ArrecadacaoLineLen)
	}
	if err != nil {
		return "", err
	}
	return b.Barcode, nil
}

func parseBarcode(code string) (*Boleto, error) {
	if code[0] == arrecadacaoProduct {
		return parseArrecadacaoBarcode(code)
	}
	return parseBancarioBarcode(code)
}

func parseLine(line string) (*Boleto, error) {
	switch len(line) {
	case BancarioLineLen:
		return parseBancarioLine(line)
	case ArrecadacaoLineLen:
		return parseArrecadacaoLine(line)
	default:
		return nil, lengthError(len(line))
	}
}

// clean strips the accepted separators and rejects anything else. It stops
// early once the input exceeds the longest valid form, bounding work on
// hostile input.
func clean(s string) (string, error) {
	b := make([]byte, 0, maxInputDigits)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			if len(b) == maxInputDigits {
				return "", fmt.Errorf("%w: more than %d digits", ErrLength, maxInputDigits)
			}
			b = append(b, c)
		case c == ' ' || c == '.' || c == '-' || c == '\t' || c == '\n' || c == '\r':
			// separator
		default:
			r, _ := utf8.DecodeRuneInString(s[i:])
			return "", fmt.Errorf("%w %q at byte offset %d", ErrCharacters, r, i)
		}
	}
	return string(b), nil
}

func lengthError(n int) error {
	return fmt.Errorf("%w: got %d digits, want %d, %d or %d", ErrLength, n, BarcodeLen, BancarioLineLen, ArrecadacaoLineLen)
}

func formatLine(d string) string {
	switch len(d) {
	case BancarioLineLen:
		return d[0:5] + "." + d[5:10] + " " + d[10:15] + "." + d[15:21] + " " +
			d[21:26] + "." + d[26:32] + " " + d[32:33] + " " + d[33:47]
	case ArrecadacaoLineLen:
		return d[0:11] + "-" + d[11:12] + " " + d[12:23] + "-" + d[23:24] + " " +
			d[24:35] + "-" + d[35:36] + " " + d[36:47] + "-" + d[47:48]
	default:
		return d
	}
}
