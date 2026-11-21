// Package report turns a brpay.Result into a flat, JSON-friendly view with
// stable field names. It is shared by the CLI and the WebAssembly build so
// that both emit the same JSON.
package report

import (
	"errors"
	"strings"
	"time"

	"github.com/victorcondackdev/brpay"
	"github.com/victorcondackdev/brpay/boleto"
	"github.com/victorcondackdev/brpay/pix"
)

// Report is the JSON view of a decode. Exactly one of the payload fields
// is set when Valid is true.
type Report struct {
	Kind        string       `json:"kind"`
	Valid       bool         `json:"valid"`
	Error       string       `json:"error,omitempty"`
	Pix         *Pix         `json:"pix,omitempty"`
	Boleto      *Boleto      `json:"boleto,omitempty"`
	Document    *Document    `json:"document,omitempty"`
	PixKey      *PixKey      `json:"pix_key,omitempty"`
	EndToEndID  *EndToEndID  `json:"end_to_end_id,omitempty"`
	Issues      []Issue      `json:"issues,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// Pix is a decoded BR Code.
type Pix struct {
	Initiation   string            `json:"initiation"` // "static" or "dynamic"
	Reusable     bool              `json:"reusable"`
	Key          string            `json:"key,omitempty"`
	KeyType      string            `json:"key_type,omitempty"`
	URL          string            `json:"url,omitempty"`
	Description  string            `json:"description,omitempty"`
	MerchantName string            `json:"merchant_name"`
	MerchantCity string            `json:"merchant_city"`
	PostalCode   string            `json:"postal_code,omitempty"`
	MCC          string            `json:"mcc"`
	Currency     string            `json:"currency"`
	Amount       string            `json:"amount,omitempty"`
	AmountCents  int64             `json:"amount_cents,omitempty"`
	TxID         string            `json:"txid,omitempty"`
	CRC          string            `json:"crc"`
	Templates    map[string]string `json:"templates,omitempty"`
}

// Boleto is a decoded boleto. Bancário-only and arrecadação-only fields are
// omitted for the other family.
type Boleto struct {
	Type          string `json:"type"` // "bancario" or "arrecadacao"
	Barcode       string `json:"barcode"`
	DigitableLine string `json:"digitable_line"`
	Formatted     string `json:"formatted"`
	AmountCents   int64  `json:"amount_cents"`

	BankCode  string `json:"bank_code,omitempty"`
	BankName  string `json:"bank_name,omitempty"`
	Currency  int    `json:"currency,omitempty"`
	Factor    int    `json:"factor,omitempty"`
	DueDate   string `json:"due_date,omitempty"` // YYYY-MM-DD
	FreeField string `json:"free_field"`

	Segment          int    `json:"segment,omitempty"`
	SegmentName      string `json:"segment_name,omitempty"`
	ValueType        int    `json:"value_type,omitempty"`
	ValueIsReference bool   `json:"value_is_reference,omitempty"`
	Modulus          int    `json:"modulus,omitempty"`
	CompanyID        string `json:"company_id,omitempty"`
	CNPJBase         string `json:"cnpj_base,omitempty"`
}

// Document is a CPF or CNPJ.
type Document struct {
	Type         string `json:"type"` // "cpf" or "cnpj"
	Number       string `json:"number"`
	Formatted    string `json:"formatted"`
	Masked       string `json:"masked"`
	Alphanumeric bool   `json:"alphanumeric"`
}

// PixKey is a normalized Pix key.
type PixKey struct {
	Type string `json:"type"` // "email", "phone" or "evp"
	Key  string `json:"key"`
}

// EndToEndID is a parsed Pix EndToEndId.
type EndToEndID struct {
	ISPB     string `json:"ispb"`
	Time     string `json:"time"` // RFC 3339, UTC
	Sequence string `json:"sequence"`
}

// Issue is a pix.Lint finding.
type Issue struct {
	Severity string `json:"severity"` // "error" or "warning"
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// Suggestion is a corrected linha digitável.
type Suggestion struct {
	Line      string `json:"line"`
	Formatted string `json:"formatted"`
}

// Decode decodes input with brpay.Decode and builds its report. now is the
// reference for resolving boleto due dates.
func Decode(input string, now time.Time) Report {
	r, err := brpay.Decode(input)
	return New(r, err, now)
}

// New builds the report of a brpay.Decode outcome.
func New(r *brpay.Result, err error, now time.Time) Report {
	rep := Report{Kind: r.Kind.String(), Valid: err == nil}
	if err != nil {
		rep.Error = err.Error()
	}
	if c := r.Pix; c != nil {
		p := &Pix{
			Initiation: "static", Reusable: c.Reusable, Key: c.Key, URL: c.URL,
			Description: c.Description, MerchantName: c.MerchantName, MerchantCity: c.MerchantCity,
			PostalCode: c.PostalCode, MCC: c.MCC, Currency: c.Currency, Amount: c.Amount,
			AmountCents: c.Cents, TxID: c.TxID, CRC: c.CRC,
		}
		if c.Dynamic {
			p.Initiation = "dynamic"
		}
		if c.KeyType != pix.KeyUnknown {
			p.KeyType = strings.ToLower(c.KeyType.String())
		}
		// Only report templates other than the Pix one.
		for id, gui := range c.Templates {
			if id != c.MerchantAccountID {
				if p.Templates == nil {
					p.Templates = map[string]string{}
				}
				p.Templates[id] = gui
			}
		}
		rep.Pix = p
	}
	if b := r.Boleto; b != nil {
		rep.Boleto = newBoleto(b, now)
	}
	if d := r.Document; d != nil {
		rep.Document = &Document{
			Type: strings.ToLower(d.Kind.String()), Number: d.Number, Formatted: d.Formatted,
			Masked: d.Masked, Alphanumeric: d.Alphanumeric,
		}
	}
	if k := r.PixKey; k != nil {
		rep.PixKey = &PixKey{Type: strings.ToLower(k.Type.String()), Key: k.Key}
	}
	if e := r.EndToEndID; e != nil {
		rep.EndToEndID = &EndToEndID{ISPB: e.ISPB, Time: e.Time.UTC().Format(time.RFC3339), Sequence: e.Sequence}
	}
	for _, is := range r.Issues {
		rep.Issues = append(rep.Issues, Issue{Severity: is.Severity.String(), Field: is.Field, Message: is.Message})
	}
	for _, s := range r.Suggestions {
		sg := Suggestion{Line: s, Formatted: s}
		if b, err := boleto.Parse(s); err == nil {
			sg.Formatted = b.Formatted()
		}
		rep.Suggestions = append(rep.Suggestions, sg)
	}
	return rep
}

func newBoleto(b *boleto.Boleto, now time.Time) *Boleto {
	out := &Boleto{
		Barcode: b.Barcode, DigitableLine: b.DigitableLine, Formatted: b.Formatted(), AmountCents: b.AmountCents,
	}
	if bk := b.Bancario; bk != nil {
		out.Type = "bancario"
		out.BankCode, out.BankName, out.Currency, out.Factor, out.FreeField = bk.BankCode, bk.BankName, bk.Currency, bk.Factor, bk.FreeField
		if due, ok := bk.DueDateAt(now); ok {
			out.DueDate = due.Format(time.DateOnly)
		}
	}
	if a := b.Arrecadacao; a != nil {
		out.Type = "arrecadacao"
		out.Segment, out.SegmentName = int(a.Segment), a.Segment.String()
		out.ValueType, out.ValueIsReference, out.Modulus = a.ValueType, a.ValueIsReference, a.Modulus
		out.CompanyID, out.CNPJBase, out.FreeField = a.CompanyID, a.CNPJBase, a.FreeField
	}
	return out
}

// Static is the outcome of building a static Pix payload.
type Static struct {
	Payload string `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ErrAmount is returned by ParseCents for malformed amounts.
var ErrAmount = errors.New("invalid amount: use digits with up to two decimals, e.g. 10.50")

// ParseCents parses a decimal amount in reais ("10", "10.5", "10.50" or,
// as typed by Brazilian users, "10,50") into cents, without floating
// point. Thousands separators are not accepted. The empty string is 0.
func ParseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	whole, frac, hasFrac := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	if whole == "" || len(whole) > 12 || len(frac) > 2 || hasFrac && frac == "" {
		return 0, ErrAmount
	}
	var cents int64
	for _, c := range whole + (frac + "00")[:2] {
		if c < '0' || c > '9' {
			return 0, ErrAmount
		}
		cents = cents*10 + int64(c-'0')
	}
	return cents, nil
}

// StaticOptions are the inputs of BuildStatic, as typed by a user.
type StaticOptions struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	City        string `json:"city"`
	Amount      string `json:"amount"` // reais, e.g. "10.50"; empty for an open amount
	TxID        string `json:"txid"`
	Description string `json:"description"`
}

// BuildStatic builds a static Pix payload with pix.NewStatic.
func BuildStatic(o StaticOptions) Static {
	cents, err := ParseCents(o.Amount)
	if err != nil {
		return Static{Error: err.Error()}
	}
	p, err := pix.NewStatic(pix.StaticOptions{
		Key: o.Key, MerchantName: o.Name, MerchantCity: o.City, Cents: cents, TxID: o.TxID, Description: o.Description,
	})
	if err != nil {
		return Static{Error: err.Error()}
	}
	return Static{Payload: p}
}
