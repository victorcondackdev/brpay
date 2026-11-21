package pix

import (
	"fmt"
	"strings"

	"github.com/victorcondackdev/brpay/emv"
)

// GUI is the globally unique identifier of the Pix merchant account
// template. Decode matches it case-insensitively.
const GUI = "br.gov.bcb.pix"

// MaxPayloadLength is the maximum length of a BR Code payload.
const MaxPayloadLength = 512

// Point of initiation method values (field 01).
const (
	InitiationReusable  = "11" // static or reusable code
	InitiationSingleUse = "12" // single-use code
)

// noTxID is the field 62.05 value meaning "no transaction identifier".
const noTxID = "***"

// BRCode is a decoded Pix BR Code.
type BRCode struct {
	// PointOfInitiation is the raw value of field 01 ("11", "12") or "" when
	// absent.
	PointOfInitiation string
	// Reusable is false only when PointOfInitiation is "12" (single use).
	Reusable bool

	// Static is true when the code carries a Pix key (subfield 01): the
	// payment data is all in the payload.
	Static bool
	// Dynamic is true when the code carries a URL (subfield 25): the payment
	// data must be fetched from the PSP. Exactly one of Static and Dynamic
	// is true.
	Dynamic bool

	// MerchantAccountID is the ID (26–51) of the template whose GUI is
	// br.gov.bcb.pix.
	MerchantAccountID string
	// Key is the Pix key exactly as written in the payload (static codes).
	Key string
	// KeyType is the detected type of Key, or KeyUnknown if Key is empty or
	// not a valid key.
	KeyType KeyType
	// Description is the free-text additional information (subfield 02).
	Description string
	// URL is the payload location without scheme (dynamic codes), e.g.
	// "pix.example.com/qr/v2/9d36b84f". It is fetched over HTTPS.
	URL string

	// MCC is the merchant category code (field 52), "0000" if unspecified.
	MCC string
	// Currency is the ISO 4217 numeric code (field 53), "986" for BRL.
	Currency string
	// Amount is the raw transaction amount (field 54), "" when absent,
	// meaning the payer chooses the amount.
	Amount string
	// Cents is Amount in cents; 0 when Amount is absent.
	Cents int64
	// CountryCode is the ISO 3166-1 alpha-2 country code (field 58).
	CountryCode string
	// MerchantName is the receiver name (field 59).
	MerchantName string
	// MerchantCity is the receiver city (field 60).
	MerchantCity string
	// PostalCode is the receiver postal code (field 61), "" when absent.
	PostalCode string
	// TxID is the transaction identifier (field 62, subfield 05); "" when
	// absent or "***".
	TxID string
	// CRC is the checksum (field 63) as written.
	CRC string

	// Templates maps the ID of every merchant account (26–51) and
	// unreserved (80–99) template to its GUI (subfield 00, "" if missing),
	// so templates of other schemes or of future BCB products are visible
	// even though this package does not interpret them.
	Templates map[string]string
	// Fields holds every parsed field in payload order, including those
	// this package does not interpret.
	Fields []emv.Field
}

// Template returns the top-level field with the given ID, with its
// subfields in Children.
func (b *BRCode) Template(id string) (emv.Field, bool) { return emv.Find(b.Fields, id) }

// Decode parses and validates a Pix "copia e cola" payload. Surrounding
// whitespace is ignored. It verifies TLV syntax, structure and CRC (errors
// from package emv are returned as is), requires a br.gov.bcb.pix merchant
// account template with either a key or a URL, the fields 52, 53, 58, 59 and
// 60, and a well-formed amount if field 54 is present.
//
// Decode does not judge values it can carry verbatim (an unusual key, a
// currency other than 986, a long name); use Lint for that.
func Decode(payload string) (*BRCode, error) {
	p := strings.TrimSpace(payload)
	fields, err := emv.Parse(p)
	if err != nil {
		return nil, err
	}
	if err := emv.Validate(fields); err != nil {
		return nil, err
	}
	if err := emv.VerifyCRC(p); err != nil {
		return nil, err
	}

	b := &BRCode{Fields: fields, Templates: map[string]string{}}
	var seen [100]bool
	for _, f := range fields {
		n := int(f.ID[0]-'0')*10 + int(f.ID[1]-'0') // Validate checked the ID
		seen[n] = true
		switch {
		case n == 1:
			b.PointOfInitiation = f.Value
		case n >= 26 && n <= 51, n >= 80:
			gui, _ := emv.Find(f.Children, "00")
			b.Templates[f.ID] = gui.Value
			if n <= 51 && b.MerchantAccountID == "" && strings.EqualFold(gui.Value, GUI) {
				b.MerchantAccountID = f.ID
				b.readAccount(f.Children)
			}
		case n == 52:
			b.MCC = f.Value
		case n == 53:
			b.Currency = f.Value
		case n == 54:
			c, err := parseAmount(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w %q: %w", ErrInvalidAmount, f.Value, err)
			}
			b.Amount, b.Cents = f.Value, c
		case n == 58:
			b.CountryCode = f.Value
		case n == 59:
			b.MerchantName = f.Value
		case n == 60:
			b.MerchantCity = f.Value
		case n == 61:
			b.PostalCode = f.Value
		case n == 62:
			if t, ok := emv.Find(f.Children, "05"); ok && t.Value != noTxID {
				b.TxID = t.Value
			}
		case n == 63:
			b.CRC = f.Value
		}
	}

	switch {
	case b.MerchantAccountID == "":
		return nil, ErrNotPix
	case b.Key != "" && b.URL != "":
		return nil, ErrKeyAndURL
	case b.Key == "" && b.URL == "":
		return nil, ErrNoKeyOrURL
	}
	for _, r := range requiredFields {
		if !seen[r.n] {
			return nil, fmt.Errorf("%w: %02d (%s)", ErrMissingField, r.n, r.name)
		}
	}
	b.Static, b.Dynamic = b.Key != "", b.URL != ""
	b.Reusable = b.PointOfInitiation != InitiationSingleUse
	if t, err := DetectKey(b.Key); err == nil {
		b.KeyType = t
	}
	return b, nil
}

var requiredFields = []struct {
	n    int
	name string
}{
	{52, "merchant category code"},
	{53, "transaction currency"},
	{58, "country code"},
	{59, "merchant name"},
	{60, "merchant city"},
}

func (b *BRCode) readAccount(sub []emv.Field) {
	for _, s := range sub {
		switch s.ID {
		case "01":
			b.Key = s.Value
		case "02":
			b.Description = s.Value
		case "25":
			b.URL = s.Value
		}
	}
}
