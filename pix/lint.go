package pix

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/victorcondackdev/brpay/emv"
)

// Severity classifies an Issue.
type Severity int

// Severities, from least to most serious.
const (
	// SeverityWarning marks something that is valid but likely to cause
	// trouble with some PSPs or that is not in canonical form.
	SeverityWarning Severity = iota + 1
	// SeverityError marks something that violates the specification and
	// that banking apps are expected to reject.
	SeverityError
)

// String returns "warning" or "error".
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Issue is a problem found by Lint.
type Issue struct {
	Severity Severity
	// Field is the field ID, as a dotted path for subfields ("26.01",
	// "62.05"), or "" for the payload as a whole.
	Field string
	// Message explains the problem and, where possible, the fix.
	Message string
}

// String formats the issue as "error [54]: message".
func (i Issue) String() string {
	if i.Field == "" {
		return i.Severity.String() + ": " + i.Message
	}
	return i.Severity.String() + " [" + i.Field + "]: " + i.Message
}

// Lint checks a BR Code payload and returns every issue it finds, in a
// stable order, or nil if there are none. It never fails: when the TLV
// cannot be parsed, it reports the positional parse error (and the CRC
// check, which does not need parsing) and stops there.
//
// Checks include: whitespace around the payload, length over 512, CRC value
// (with the expected value) and case, payload structure, the
// br.gov.bcb.pix template (key xor URL, key validity and canonical form, URL
// without scheme), MCC, currency 986, amount syntax (comma, decimals,
// leading zeros, zero), country BR, merchant name (25) and city (15)
// limits, postal code, txid charset and length, and non-ASCII characters in
// any field.
func Lint(payload string) []Issue {
	l := &linter{}
	p := strings.TrimSpace(payload)
	if p != payload {
		l.warn("", "payload has surrounding whitespace or line breaks (often a copy-paste artifact); some apps reject it")
	}
	if p == "" {
		l.error("", "payload is empty")
		return l.issues
	}
	if len(p) > MaxPayloadLength {
		l.error("", fmt.Sprintf("payload is %d bytes, max %d", len(p), MaxPayloadLength))
	}
	l.checkCRC(p)

	fields, err := emv.Parse(p)
	if err != nil {
		var pe *emv.ParseError
		id := ""
		if errors.As(err, &pe) {
			id = pe.ID
		}
		msg := err.Error()
		if !isPrintableASCII(p) {
			msg += " (the payload contains non-ASCII characters; lengths must count bytes, and BR Codes should be ASCII only)"
		}
		l.error(id, msg)
		return l.issues
	}
	if err := emv.Validate(fields); err != nil {
		var pe *emv.ParseError
		if errors.As(err, &pe) {
			l.error(pe.ID, pe.Reason)
		}
	}
	l.checkText(fields, "")
	l.checkFields(fields)
	return l.issues
}

type linter struct{ issues []Issue }

func (l *linter) error(field, msg string) {
	l.issues = append(l.issues, Issue{Severity: SeverityError, Field: field, Message: msg})
}

func (l *linter) warn(field, msg string) {
	l.issues = append(l.issues, Issue{Severity: SeverityWarning, Field: field, Message: msg})
}

func (l *linter) checkCRC(p string) {
	n := len(p)
	if n < 8 || p[n-8:n-4] != emv.CRCPrefix {
		l.error("63", "payload must end with the CRC field: \"6304\" followed by 4 hex digits")
		return
	}
	got, want := p[n-4:], emv.CRC16(p[:n-4])
	switch {
	case !strings.EqualFold(got, want):
		l.error("63", fmt.Sprintf("CRC is %q, expected %s (was the payload edited after the CRC was computed?)", got, want))
	case got != want:
		l.warn("63", fmt.Sprintf("CRC hex digits should be uppercase: %s", want))
	}
}

// checkText reports values that are not printable ASCII.
func (l *linter) checkText(fields []emv.Field, parent string) {
	for _, f := range fields {
		path := f.ID
		if parent != "" {
			path = parent + "." + f.ID
		}
		if len(f.Children) > 0 {
			l.checkText(f.Children, path)
			continue
		}
		for i := 0; i < len(f.Value); i++ {
			if c := f.Value[i]; c < 0x20 || c > 0x7E {
				r, _ := utf8.DecodeRuneInString(f.Value[i:])
				l.error(path, fmt.Sprintf("non-ASCII or control character %q at byte %d of the value; transliterate accents (\"São\" to \"Sao\")", r, i))
				break
			}
		}
	}
}

func (l *linter) checkFields(fields []emv.Field) {
	if f, ok := emv.Find(fields, "01"); ok && f.Value != InitiationReusable && f.Value != InitiationSingleUse {
		l.error("01", fmt.Sprintf("point of initiation method is %q, want \"11\" (reusable) or \"12\" (single use)", f.Value))
	}
	l.checkAccount(fields)

	if f, ok := l.require(fields, "52", "merchant category code"); ok && (len(f.Value) != 4 || !isDigits(f.Value)) {
		l.error("52", fmt.Sprintf("merchant category code %q must be 4 digits (\"0000\" if unspecified)", f.Value))
	}
	if f, ok := l.require(fields, "53", "transaction currency"); ok && f.Value != "986" {
		l.error("53", fmt.Sprintf("currency is %q, Pix requires \"986\" (BRL)", f.Value))
	}
	if f, ok := emv.Find(fields, "54"); ok {
		l.checkAmount(f.Value)
	}
	if f, ok := l.require(fields, "58", "country code"); ok && f.Value != "BR" {
		l.error("58", fmt.Sprintf("country code is %q, want \"BR\"", f.Value))
	}
	if f, ok := l.require(fields, "59", "merchant name"); ok && len(f.Value) > MaxMerchantNameLength {
		l.error("59", fmt.Sprintf("merchant name is %d characters, max %d", len(f.Value), MaxMerchantNameLength))
	}
	if f, ok := l.require(fields, "60", "merchant city"); ok && len(f.Value) > MaxMerchantCityLength {
		l.error("60", fmt.Sprintf("merchant city is %d characters, max %d", len(f.Value), MaxMerchantCityLength))
	}
	if f, ok := emv.Find(fields, "61"); ok && (len(f.Value) != 8 || !isDigits(f.Value)) {
		l.warn("61", fmt.Sprintf("postal code %q should be a CEP of 8 digits without hyphen", f.Value))
	}
	l.checkTxID(fields)
}

func (l *linter) require(fields []emv.Field, id, name string) (emv.Field, bool) {
	f, ok := emv.Find(fields, id)
	if !ok {
		l.error(id, "missing required field ("+name+")")
	}
	return f, ok
}

func (l *linter) checkAccount(fields []emv.Field) {
	var acct emv.Field
	for _, f := range fields {
		if n := f.ID; n >= "26" && n <= "51" {
			if gui, _ := emv.Find(f.Children, "00"); strings.EqualFold(gui.Value, GUI) {
				acct = f
				if gui.Value != GUI {
					l.warn(f.ID+".00", fmt.Sprintf("GUI %q should be lowercase %q", gui.Value, GUI))
				}
				break
			}
		}
	}
	if acct.ID == "" {
		l.error("26", "no merchant account template (26–51) with GUI \"br.gov.bcb.pix\"")
		return
	}
	key, hasKey := emv.Find(acct.Children, "01")
	url, hasURL := emv.Find(acct.Children, "25")
	switch {
	case hasKey && hasURL:
		l.error(acct.ID, "has both a key (01) and a URL (25); a BR Code is either static or dynamic")
	case !hasKey && !hasURL:
		l.error(acct.ID, "has neither a key (01) nor a URL (25)")
	}
	if hasKey {
		if norm, _, err := NormalizeKey(key.Value); err != nil {
			l.error(acct.ID+".01", err.Error())
		} else if norm != key.Value {
			l.warn(acct.ID+".01", fmt.Sprintf("key is not in canonical form; use %q", norm))
		}
	}
	if hasURL {
		if strings.Contains(url.Value, "://") {
			l.error(acct.ID+".25", "URL must not include the scheme (remove \"https://\")")
		}
	}
}

func (l *linter) checkAmount(v string) {
	cents, err := parseAmount(v)
	if err != nil {
		l.error("54", fmt.Sprintf("amount %q: %v", v, err))
		return
	}
	if len(v) > 1 && v[0] == '0' && v[1] != '.' {
		l.warn("54", fmt.Sprintf("amount %q has leading zeros; use %q", v, formatAmount(cents)))
	}
	if cents == 0 {
		l.warn("54", "amount is zero; omit field 54 to let the payer choose the amount")
	}
}

func (l *linter) checkTxID(fields []emv.Field) {
	add, ok := emv.Find(fields, "62")
	if !ok {
		l.warn("62", "missing additional data template; many PSPs expect 62 with subfield 05 (txid, \"***\" for none)")
		return
	}
	t, ok := emv.Find(add.Children, "05")
	switch {
	case !ok:
		l.warn("62.05", "missing txid; use \"***\" for none")
	case t.Value == noTxID:
	default:
		if err := validateTxID(t.Value); err != nil {
			l.error("62.05", strings.TrimPrefix(err.Error(), "pix: "))
		}
	}
}
