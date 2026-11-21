package brpay

import (
	"errors"
	"strings"

	"github.com/victorcondackdev/brpay/boleto"
	"github.com/victorcondackdev/brpay/document"
	"github.com/victorcondackdev/brpay/pix"
)

// Kind is the type of input recognized by [Detect].
type Kind int

// Kinds of input, in no particular order.
const (
	KindUnknown    Kind = iota
	KindPix             // Pix BR Code ("copia e cola")
	KindBoleto          // boleto barcode or linha digitável
	KindDocument        // CPF or CNPJ
	KindPixKey          // Pix key: e-mail, phone or EVP (random key)
	KindEndToEndID      // Pix EndToEndId
)

var kindNames = [...]string{
	KindUnknown:    "unknown",
	KindPix:        "pix",
	KindBoleto:     "boleto",
	KindDocument:   "document",
	KindPixKey:     "pix_key",
	KindEndToEndID: "end_to_end_id",
}

// String returns a stable lowercase name: "pix", "boleto", "document",
// "pix_key", "end_to_end_id" or "unknown".
func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return kindNames[KindUnknown]
	}
	return kindNames[k]
}

// MarshalText implements [encoding.TextMarshaler] using [Kind.String], so
// Kind encodes as a JSON string.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// ErrUnrecognized is returned by [Decode] when the input does not look like
// anything this module understands.
var ErrUnrecognized = errors.New("brpay: unrecognized input")

// Document is a validated CPF or CNPJ.
type Document struct {
	Kind document.Kind
	// Number is the bare identifier: 11 digits, or 14 characters for a CNPJ
	// (uppercase letters allowed in the first 12).
	Number string
	// Formatted is the punctuated form, e.g. 529.982.247-25.
	Formatted string
	// Masked hides the middle of the number for logs, e.g. ***.982.247-**.
	Masked string
	// Alphanumeric reports a CNPJ in the alphanumeric format.
	Alphanumeric bool
}

// PixKey is a validated Pix key that is not a CPF or CNPJ (those decode as
// [KindDocument]).
type PixKey struct {
	Type pix.KeyType
	// Key is the canonical form used in the DICT and in BR Codes.
	Key string
}

// Result is the outcome of [Decode]. Kind says which one of Pix, Boleto,
// Document, PixKey and EndToEndID is set; on error that field is nil but
// Kind, Issues and Suggestions are still filled in.
type Result struct {
	Kind Kind

	Pix        *pix.BRCode
	Boleto     *boleto.Boleto
	Document   *Document
	PixKey     *PixKey
	EndToEndID *pix.EndToEndID

	// Issues are the findings of [pix.Lint] for KindPix, computed even when
	// the payload does not decode. A payload can decode and still have
	// issues (warnings, or errors Decode is lenient about).
	Issues []pix.Issue
	// Suggestions are corrected linhas digitáveis from [boleto.SuggestFix]
	// when a linha digitável fails a check digit.
	Suggestions []string
}

// Detect classifies input by its shape, without fully validating it. The
// rules, applied to the input with surrounding whitespace removed, in order:
//
//  1. Digits with optional spaces, dots and hyphens, 44, 47 or 48 of them,
//     that form a valid boleto: KindBoleto. A valid boleto wins over
//     everything else, including the "000201" Pix prefix.
//  2. Starts with "000201" (the BR Code payload format indicator): KindPix.
//  3. Same shape as rule 1 but with a check-digit or layout error:
//     KindBoleto, so that Decode reports the boleto error and suggestions.
//  4. 11 digits, or 14 characters in CNPJ shape (numeric or alphanumeric),
//     after removing ".", "-", "/" and spaces: KindDocument. A CPF or CNPJ
//     is also a valid Pix key; it is reported as a document because that
//     is what an 11- or 14-character identifier usually is.
//  5. 32 characters starting with "E": KindEndToEndID.
//  6. Contains "@", starts with "+", or has the UUID shape: KindPixKey.
//
// Anything else is KindUnknown.
func Detect(input string) Kind {
	s := strings.TrimSpace(input)
	if s == "" {
		return KindUnknown
	}
	boletoShape := isBoletoShape(s)
	switch {
	case boletoShape && boleto.Validate(s) == nil:
		return KindBoleto
	case strings.HasPrefix(s, "000201"):
		return KindPix
	case boletoShape:
		return KindBoleto
	case isDocumentShape(s):
		return KindDocument
	case len(s) == pix.EndToEndIDLength && s[0] == 'E':
		return KindEndToEndID
	case strings.Contains(s, "@") || s[0] == '+' || isUUIDShape(s):
		return KindPixKey
	}
	return KindUnknown
}

// Decode detects the kind of input (see [Detect]) and decodes it with the
// matching package. It always returns a non-nil *Result. The error, if
// any, comes unwrapped from the underlying package, so errors.Is and
// errors.As work with its sentinel and structured errors (for example
// [boleto.ErrCheckDigit], [*boleto.CheckDigitError] and
// [*github.com/victorcondackdev/brpay/emv.ParseError]). Input that is not
// recognized yields [ErrUnrecognized].
func Decode(input string) (*Result, error) {
	r := &Result{Kind: Detect(input)}
	s := strings.TrimSpace(input)
	var err error
	switch r.Kind {
	case KindPix:
		r.Issues = pix.Lint(input)
		r.Pix, err = pix.Decode(s)
	case KindBoleto:
		r.Boleto, err = boleto.Parse(s)
		if errors.Is(err, boleto.ErrCheckDigit) {
			r.Suggestions = boleto.SuggestFix(s)
		}
	case KindDocument:
		r.Document, err = decodeDocument(s)
	case KindEndToEndID:
		var id pix.EndToEndID
		if id, err = pix.ParseEndToEndID(s); err == nil {
			r.EndToEndID = &id
		}
	case KindPixKey:
		var k PixKey
		if k.Key, k.Type, err = pix.NormalizeKey(s); err == nil {
			r.PixKey = &k
		}
	default:
		err = ErrUnrecognized
	}
	return r, err
}

func decodeDocument(s string) (*Document, error) {
	n := document.Clean(s)
	var err error
	k := document.CPF
	if len(n) == 11 {
		err = document.ValidateCPF(n)
	} else {
		k = document.CNPJ
		err = document.ValidateCNPJ(n)
	}
	if err != nil {
		return nil, err
	}
	return &Document{
		Kind:         k,
		Number:       n,
		Formatted:    document.Format(n),
		Masked:       document.Mask(n),
		Alphanumeric: document.IsAlphanumericCNPJ(n),
	}, nil
}

// isBoletoShape reports whether s is 44, 47 or 48 digits once the
// separators boleto.Parse ignores are removed.
func isBoletoShape(s string) bool {
	n := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			n++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '.' || c == '-':
		default:
			return false
		}
	}
	return n == boleto.BarcodeLen || n == boleto.BancarioLineLen || n == boleto.ArrecadacaoLineLen
}

// isDocumentShape reports whether s is 11 digits (CPF) or 12 alphanumeric
// characters followed by 2 digits (CNPJ) once the usual punctuation is
// removed. Lowercase letters are accepted, as document.Clean uppercases.
func isDocumentShape(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isAlnum(c) && c != '.' && c != '-' && c != '/' && c != ' ' {
			return false
		}
	}
	c := document.Clean(s)
	switch len(c) {
	case 11:
		return isDigits(c)
	case 14:
		for i := 0; i < 12; i++ {
			if !isAlnum(c[i]) {
				return false
			}
		}
		return isDigits(c[12:])
	}
	return false
}

func isUUIDShape(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if s[i] != '-' {
				return false
			}
		} else if !isAlnum(s[i]) {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
