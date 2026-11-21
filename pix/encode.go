package pix

import (
	"fmt"
	"strings"

	"github.com/victorcondackdev/brpay/emv"
)

// Field limits enforced by NewStatic and NewDynamic.
const (
	MaxMerchantNameLength = 25
	MaxMerchantCityLength = 15
	MaxTxIDLength         = 25
)

// StaticOptions describes a static BR Code, which carries a Pix key and can
// be paid any number of times.
type StaticOptions struct {
	// Key is the receiver's Pix key, in any format NormalizeKey accepts.
	Key string
	// MerchantName is the receiver name, at most 25 characters.
	MerchantName string
	// MerchantCity is the receiver city, at most 15 characters.
	MerchantCity string
	// Cents is the amount in cents. Zero leaves the amount open for the
	// payer to fill in.
	Cents int64
	// TxID identifies the payment for reconciliation: up to 25 characters
	// from [A-Za-z0-9]. Empty means none ("***").
	TxID string
	// Description is optional free text shown to the payer. Its length is
	// bounded by the 99-byte merchant account template it shares with Key.
	Description string
	// PostalCode is the optional receiver CEP, 8 digits; a hyphen is allowed.
	PostalCode string
	// MCC is the merchant category code, 4 digits; empty means "0000".
	MCC string
}

// DynamicOptions describes a dynamic BR Code, whose payment data is served
// by the receiver's PSP at URL.
type DynamicOptions struct {
	// URL is the payload location. A leading "https://" is removed, as the
	// BR Code stores the location without scheme.
	URL string
	// MerchantName is the receiver name, at most 25 characters.
	MerchantName string
	// MerchantCity is the receiver city, at most 15 characters.
	MerchantCity string
	// PostalCode is the optional receiver CEP, 8 digits; a hyphen is allowed.
	PostalCode string
	// MCC is the merchant category code, 4 digits; empty means "0000".
	MCC string
	// Reusable marks the code as reusable (field 01 = "11"). By default the
	// code is single-use (field 01 = "12").
	Reusable bool
}

// NewStatic builds a static BR Code payload, CRC included.
//
// Text fields (MerchantName, MerchantCity, Description) are always trimmed
// and Portuguese accented letters are always transliterated to ASCII
// ("São Paulo" becomes "Sao Paulo"); any other non-ASCII character is an
// error. Limits are checked after transliteration. The key is stored in its
// NormalizeKey form. Field 01 (point of initiation) is omitted, which means
// reusable.
func NewStatic(opts StaticOptions) (string, error) {
	key, _, err := NormalizeKey(opts.Key)
	if err != nil {
		return "", err
	}
	desc, err := toASCII(opts.Description)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidDescription, err)
	}
	account := []emv.Field{{ID: "00", Value: GUI}, {ID: "01", Value: key}}
	if desc != "" {
		account = append(account, emv.Field{ID: "02", Value: desc})
	}
	if n := templateLen(account); n > emv.MaxValueLength {
		return "", fmt.Errorf("%w: key and description take %d bytes in the merchant account template, max %d", ErrInvalidDescription, n, emv.MaxValueLength)
	}

	if opts.Cents < 0 || opts.Cents > MaxCents {
		return "", fmt.Errorf("%w: %d cents is out of range", ErrInvalidAmount, opts.Cents)
	}
	txid := opts.TxID
	if txid == "" {
		txid = noTxID
	} else if err := validateTxID(txid); err != nil {
		return "", err
	}
	var amount string
	if opts.Cents > 0 {
		amount = formatAmount(opts.Cents)
	}
	return build("", account, amount, txid, opts.MerchantName, opts.MerchantCity, opts.PostalCode, opts.MCC)
}

// NewDynamic builds a dynamic BR Code payload, CRC included. Text fields
// follow the same rules as in NewStatic. The amount and txid are not part
// of a dynamic payload; they are served by the PSP at URL.
func NewDynamic(opts DynamicOptions) (string, error) {
	u := strings.TrimSpace(opts.URL)
	if len(u) >= 8 && strings.EqualFold(u[:8], "https://") {
		u = u[8:]
	}
	switch {
	case u == "":
		return "", fmt.Errorf("%w: empty", ErrInvalidURL)
	case strings.Contains(u, "://"):
		return "", fmt.Errorf("%w: only https URLs are allowed", ErrInvalidURL)
	case !isPrintableASCII(u) || strings.IndexByte(u, ' ') >= 0:
		return "", fmt.Errorf("%w: must be printable ASCII without spaces", ErrInvalidURL)
	}
	account := []emv.Field{{ID: "00", Value: GUI}, {ID: "25", Value: u}}
	if n := templateLen(account); n > emv.MaxValueLength {
		return "", fmt.Errorf("%w: %d bytes in the merchant account template, max %d", ErrInvalidURL, n, emv.MaxValueLength)
	}
	poi := InitiationSingleUse
	if opts.Reusable {
		poi = InitiationReusable
	}
	return build(poi, account, "", noTxID, opts.MerchantName, opts.MerchantCity, opts.PostalCode, opts.MCC)
}

func build(poi string, account []emv.Field, amount, txid, name, city, postal, mcc string) (string, error) {
	name, err := limitedText(name, MaxMerchantNameLength, ErrInvalidMerchantName)
	if err != nil {
		return "", err
	}
	city, err = limitedText(city, MaxMerchantCityLength, ErrInvalidMerchantCity)
	if err != nil {
		return "", err
	}
	postal = strings.ReplaceAll(strings.TrimSpace(postal), "-", "")
	if postal != "" && (len(postal) != 8 || !isDigits(postal)) {
		return "", fmt.Errorf("%w: want 8 digits", ErrInvalidPostalCode)
	}
	if mcc == "" {
		mcc = "0000"
	} else if len(mcc) != 4 || !isDigits(mcc) {
		return "", fmt.Errorf("%w: want 4 digits", ErrInvalidMCC)
	}

	fields := make([]emv.Field, 0, 11)
	fields = append(fields, emv.Field{ID: "00", Value: "01"})
	if poi != "" {
		fields = append(fields, emv.Field{ID: "01", Value: poi})
	}
	fields = append(fields,
		emv.Field{ID: "26", Children: account},
		emv.Field{ID: "52", Value: mcc},
		emv.Field{ID: "53", Value: "986"},
	)
	if amount != "" {
		fields = append(fields, emv.Field{ID: "54", Value: amount})
	}
	fields = append(fields,
		emv.Field{ID: "58", Value: "BR"},
		emv.Field{ID: "59", Value: name},
		emv.Field{ID: "60", Value: city},
	)
	if postal != "" {
		fields = append(fields, emv.Field{ID: "61", Value: postal})
	}
	fields = append(fields, emv.Field{ID: "62", Children: []emv.Field{{ID: "05", Value: txid}}})

	body, err := emv.Encode(fields)
	if err != nil {
		return "", err // unreachable: every value is checked above
	}
	p := emv.AppendCRC(body)
	if len(p) > MaxPayloadLength {
		return "", fmt.Errorf("%w: %d bytes, max %d", ErrPayloadTooLong, len(p), MaxPayloadLength)
	}
	return p, nil
}

func limitedText(s string, max int, sentinel error) (string, error) {
	t, err := toASCII(s)
	switch {
	case err != nil:
		return "", fmt.Errorf("%w: %w", sentinel, err)
	case t == "":
		return "", fmt.Errorf("%w: empty", sentinel)
	case len(t) > max:
		return "", fmt.Errorf("%w: %d characters, max %d", sentinel, len(t), max)
	}
	return t, nil
}

func validateTxID(s string) error {
	if len(s) > MaxTxIDLength {
		return fmt.Errorf("%w: %d characters, max %d", ErrInvalidTxID, len(s), MaxTxIDLength)
	}
	for i := 0; i < len(s); i++ {
		if !isAlnum(s[i]) {
			return fmt.Errorf("%w: invalid character %q at position %d, want [A-Za-z0-9]", ErrInvalidTxID, s[i], i)
		}
	}
	return nil
}

func templateLen(fields []emv.Field) int {
	n := 0
	for _, f := range fields {
		n += 4 + len(f.Value)
	}
	return n
}
