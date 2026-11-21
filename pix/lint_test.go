package pix

import (
	"strings"
	"testing"
)

func TestLintClean(t *testing.T) {
	if issues := Lint(bcbExample); issues != nil {
		t.Errorf("Lint(bcbExample) = %v", issues)
	}
}

// lintBody is a valid static payload without CRC, built from parts so each
// test can break exactly one thing.
type lintBody struct {
	poi, acct, mcc, cur, amount, country, name, city, postal, add string
}

func okBody() lintBody {
	return lintBody{
		acct: tlv("26", tlv("00", GUI)+tlv("01", "52998224725")),
		mcc:  tlv("52", "0000"), cur: tlv("53", "986"), country: tlv("58", "BR"),
		name: tlv("59", "Fulano"), city: tlv("60", "Rio"), add: tlv("62", tlv("05", "***")),
	}
}

func (b lintBody) String() string {
	return "000201" + b.poi + b.acct + b.mcc + b.cur + b.amount + b.country + b.name + b.city + b.postal + b.add
}

func TestLint(t *testing.T) {
	tests := []struct {
		name    string
		payload func() string
		want    []string // Issue.String() values, in order
	}{
		{"whitespace", func() string { return withCRC(okBody().String()) + "\n" },
			[]string{"warning: payload has surrounding whitespace or line breaks (often a copy-paste artifact); some apps reject it"}},
		{"empty", func() string { return " " },
			[]string{"warning: payload has surrounding whitespace or line breaks (often a copy-paste artifact); some apps reject it", "error: payload is empty"}},
		{"wrong crc", func() string { return bcbExample[:len(bcbExample)-4] + "1D3E" },
			[]string{`error [63]: CRC is "1D3E", expected 1D3D (was the payload edited after the CRC was computed?)`}},
		{"lowercase crc", func() string { return bcbExample[:len(bcbExample)-4] + "1d3d" },
			[]string{"warning [63]: CRC hex digits should be uppercase: 1D3D"}},
		{"amount comma", func() string { b := okBody(); b.amount = tlv("54", "10,50"); return withCRC(b.String()) },
			[]string{`error [54]: amount "10,50": uses a comma; the decimal separator must be a dot`}},
		{"amount decimals", func() string { b := okBody(); b.amount = tlv("54", "1.234"); return withCRC(b.String()) },
			[]string{`error [54]: amount "1.234": more than 2 decimal places`}},
		{"amount leading zeros", func() string { b := okBody(); b.amount = tlv("54", "010.00"); return withCRC(b.String()) },
			[]string{`warning [54]: amount "010.00" has leading zeros; use "10.00"`}},
		{"amount zero", func() string { b := okBody(); b.amount = tlv("54", "0.00"); return withCRC(b.String()) },
			[]string{"warning [54]: amount is zero; omit field 54 to let the payer choose the amount"}},
		{"long name and city", func() string {
			b := okBody()
			b.name, b.city = tlv("59", strings.Repeat("N", 26)), tlv("60", "Sao Joao de Meriti")
			return withCRC(b.String())
		}, []string{"error [59]: merchant name is 26 characters, max 25", "error [60]: merchant city is 18 characters, max 15"}},
		{"accents", func() string { b := okBody(); b.city = tlv("60", "Niterói"); return withCRC(b.String()) },
			[]string{`error [60]: non-ASCII or control character 'ó' at byte 5 of the value; transliterate accents ("São" to "Sao")`}},
		{"missing country and currency", func() string { b := okBody(); b.country, b.cur = "", ""; return withCRC(b.String()) },
			[]string{"error [53]: missing required field (transaction currency)", "error [58]: missing required field (country code)"}},
		{"wrong country currency mcc", func() string {
			b := okBody()
			b.country, b.cur, b.mcc = tlv("58", "US"), tlv("53", "840"), tlv("52", "00")
			return withCRC(b.String())
		}, []string{
			`error [52]: merchant category code "00" must be 4 digits ("0000" if unspecified)`,
			`error [53]: currency is "840", Pix requires "986" (BRL)`,
			`error [58]: country code is "US", want "BR"`,
		}},
		{"missing name city mcc", func() string { b := okBody(); b.name, b.city, b.mcc = "", "", ""; return withCRC(b.String()) },
			[]string{
				"error [52]: missing required field (merchant category code)",
				"error [59]: missing required field (merchant name)",
				"error [60]: missing required field (merchant city)",
			}},
		{"txid", func() string { b := okBody(); b.add = tlv("62", tlv("05", "PEDIDO-1")); return withCRC(b.String()) },
			[]string{`error [62.05]: invalid txid: invalid character '-' at position 6, want [A-Za-z0-9]`}},
		{"txid long", func() string {
			b := okBody()
			b.add = tlv("62", tlv("05", strings.Repeat("A", 26)))
			return withCRC(b.String())
		},
			[]string{"error [62.05]: invalid txid: 26 characters, max 25"}},
		{"no 62", func() string { b := okBody(); b.add = ""; return withCRC(b.String()) },
			[]string{`warning [62]: missing additional data template; many PSPs expect 62 with subfield 05 (txid, "***" for none)`}},
		{"no txid", func() string { b := okBody(); b.add = tlv("62", tlv("01", "x")); return withCRC(b.String()) },
			[]string{`warning [62.05]: missing txid; use "***" for none`}},
		{"key and url", func() string {
			b := okBody()
			b.acct = tlv("26", tlv("00", GUI)+tlv("01", "52998224725")+tlv("25", "https://x.com/qr/1"))
			return withCRC(b.String())
		}, []string{
			"error [26]: has both a key (01) and a URL (25); a BR Code is either static or dynamic",
			`error [26.25]: URL must not include the scheme (remove "https://")`,
		}},
		{"no key no url", func() string { b := okBody(); b.acct = tlv("26", tlv("00", GUI)); return withCRC(b.String()) },
			[]string{"error [26]: has neither a key (01) nor a URL (25)"}},
		{"not pix", func() string {
			b := okBody()
			b.acct = tlv("27", tlv("00", "br.com.other"))
			return withCRC(b.String())
		},
			[]string{`error [26]: no merchant account template (26–51) with GUI "br.gov.bcb.pix"`}},
		{"gui case and key form", func() string {
			b := okBody()
			b.acct = tlv("26", tlv("00", "BR.GOV.BCB.PIX")+tlv("01", "529.982.247-25"))
			return withCRC(b.String())
		}, []string{
			`warning [26.00]: GUI "BR.GOV.BCB.PIX" should be lowercase "br.gov.bcb.pix"`,
			`warning [26.01]: key is not in canonical form; use "52998224725"`,
		}},
		{"invalid key", func() string {
			b := okBody()
			b.acct = tlv("26", tlv("00", GUI)+tlv("01", "12345"))
			return withCRC(b.String())
		},
			[]string{"error [26.01]: pix: invalid key: not a CPF, CNPJ, e-mail, phone (+55...) or EVP"}},
		{"poi and postal", func() string {
			b := okBody()
			b.poi, b.postal = tlv("01", "13"), tlv("61", "24400-000")
			return withCRC(b.String())
		},
			[]string{
				`error [01]: point of initiation method is "13", want "11" (reusable) or "12" (single use)`,
				`warning [61]: postal code "24400-000" should be a CEP of 8 digits without hyphen`,
			}},
		{"structure", func() string { return withCRC("000202" + okBody().String()[6:]) },
			[]string{`error [00]: payload format indicator must be "01", got "02"`}},
		{"no crc", func() string { return okBody().String() },
			[]string{
				`error [63]: payload must end with the CRC field: "6304" followed by 4 hex digits`,
				"error [62]: CRC (63) must be the last field",
			}},
		{"syntax", func() string { return "000201XX01" },
			[]string{
				`error [63]: payload must end with the CRC field: "6304" followed by 4 hex digits`,
				`error: emv: byte 6: invalid field ID "XX": want 2 digits`,
			}},
		{"rune-counted length", func() string { return withCRC("0002015907Niterói") },
			[]string{`error: emv: byte 17: invalid field ID "i6": want 2 digits (the payload contains non-ASCII characters; lengths must count bytes, and BR Codes should be ASCII only)`}},
	}
	for _, tt := range tests {
		issues := Lint(tt.payload())
		got := make([]string, len(issues))
		for i, is := range issues {
			got[i] = is.String()
		}
		if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("%s:\n got: %q\nwant: %q", tt.name, got, tt.want)
		}
	}
}

func TestLintTooLong(t *testing.T) {
	p := okBody().String()
	for _, id := range []string{"80", "81", "82", "83", "84"} {
		p += tlv(id, tlv("00", strings.Repeat("x", 91)))
	}
	issues := Lint(withCRC(p))
	if len(issues) != 1 || issues[0].String() != "error: payload is 595 bytes, max 512" {
		t.Errorf("Lint = %v", issues)
	}
}

func TestSeverityString(t *testing.T) {
	if SeverityWarning.String() != "warning" || SeverityError.String() != "error" || Severity(0).String() != "unknown" {
		t.Error("Severity.String mismatch")
	}
}
