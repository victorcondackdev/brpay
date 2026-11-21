package pix

import (
	"errors"
	"strings"
	"testing"
)

func TestNewStaticRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		opts StaticOptions
		want BRCode // only the compared fields are set
	}{
		{
			name: "minimal",
			opts: StaticOptions{Key: "123e4567-e12b-12d1-a456-426655440000", MerchantName: "Fulano de Tal", MerchantCity: "BRASILIA"},
			want: BRCode{Key: "123e4567-e12b-12d1-a456-426655440000", KeyType: KeyEVP, MerchantName: "Fulano de Tal", MerchantCity: "BRASILIA", MCC: "0000"},
		},
		{
			name: "full with accents",
			opts: StaticOptions{
				Key: "+55 (21) 98765-4321", MerchantName: "  Padaria Pão Quente ", MerchantCity: "São Gonçalo",
				Cents: 1050, TxID: "PEDIDO42", Description: "Café e pão", PostalCode: "24400-000", MCC: "5462",
			},
			want: BRCode{
				Key: "+5521987654321", KeyType: KeyPhone, MerchantName: "Padaria Pao Quente", MerchantCity: "Sao Goncalo",
				Amount: "10.50", Cents: 1050, TxID: "PEDIDO42", Description: "Cafe e pao", PostalCode: "24400000", MCC: "5462",
			},
		},
		{
			name: "cnpj one cent",
			opts: StaticOptions{Key: "12.ABC.345/01DE-35", MerchantName: "ACME LTDA", MerchantCity: "RIO", Cents: 1},
			want: BRCode{Key: "12ABC34501DE35", KeyType: KeyCNPJ, MerchantName: "ACME LTDA", MerchantCity: "RIO", Amount: "0.01", Cents: 1, MCC: "0000"},
		},
		{
			name: "max amount",
			opts: StaticOptions{Key: "52998224725", MerchantName: "X", MerchantCity: "Y", Cents: MaxCents},
			want: BRCode{Key: "52998224725", KeyType: KeyCPF, MerchantName: "X", MerchantCity: "Y", Amount: "9999999999.99", Cents: MaxCents, MCC: "0000"},
		},
	}
	for _, tt := range tests {
		p, err := NewStatic(tt.opts)
		if err != nil {
			t.Fatalf("%s: NewStatic: %v", tt.name, err)
		}
		if issues := Lint(p); len(issues) != 0 {
			t.Errorf("%s: Lint(%q) = %v", tt.name, p, issues)
		}
		b, err := Decode(p)
		if err != nil {
			t.Fatalf("%s: Decode(%q): %v", tt.name, p, err)
		}
		w := tt.want
		if !b.Static || b.Dynamic || !b.Reusable || b.Key != w.Key || b.KeyType != w.KeyType ||
			b.MerchantName != w.MerchantName || b.MerchantCity != w.MerchantCity || b.Amount != w.Amount ||
			b.Cents != w.Cents || b.TxID != w.TxID || b.Description != w.Description ||
			b.PostalCode != w.PostalCode || b.MCC != w.MCC || b.Currency != "986" || b.CountryCode != "BR" {
			t.Errorf("%s: Decode(%q) = %+v", tt.name, p, b)
		}
	}
}

func TestNewStaticMatchesBCBExample(t *testing.T) {
	p, err := NewStatic(StaticOptions{Key: "123e4567-e12b-12d1-a456-426655440000", MerchantName: "Fulano de Tal", MerchantCity: "BRASILIA"})
	if err != nil || p != bcbExample {
		t.Errorf("NewStatic = %q, %v\nwant %q", p, err, bcbExample)
	}
}

func TestNewStaticErrors(t *testing.T) {
	ok := StaticOptions{Key: "52998224725", MerchantName: "Fulano", MerchantCity: "Rio"}
	tests := []struct {
		name   string
		modify func(*StaticOptions)
		want   error
	}{
		{"key", func(o *StaticOptions) { o.Key = "123" }, ErrInvalidKey},
		{"empty name", func(o *StaticOptions) { o.MerchantName = " " }, ErrInvalidMerchantName},
		{"long name", func(o *StaticOptions) { o.MerchantName = strings.Repeat("a", 26) }, ErrInvalidMerchantName},
		{"emoji name", func(o *StaticOptions) { o.MerchantName = "Loja 🙂" }, ErrInvalidMerchantName},
		{"bad utf8 name", func(o *StaticOptions) { o.MerchantName = "Loja \xff" }, ErrInvalidMerchantName},
		{"long city", func(o *StaticOptions) { o.MerchantCity = "Sao Joao de Meriti" }, ErrInvalidMerchantCity},
		{"negative", func(o *StaticOptions) { o.Cents = -1 }, ErrInvalidAmount},
		{"too much", func(o *StaticOptions) { o.Cents = MaxCents + 1 }, ErrInvalidAmount},
		{"txid chars", func(o *StaticOptions) { o.TxID = "PEDIDO-1" }, ErrInvalidTxID},
		{"txid long", func(o *StaticOptions) { o.TxID = strings.Repeat("A", 26) }, ErrInvalidTxID},
		{"description chars", func(o *StaticOptions) { o.Description = "tab\there" }, ErrInvalidDescription},
		{"description long", func(o *StaticOptions) { o.Description = strings.Repeat("d", 70) }, ErrInvalidDescription},
		{"postal", func(o *StaticOptions) { o.PostalCode = "1234" }, ErrInvalidPostalCode},
		{"postal letters", func(o *StaticOptions) { o.PostalCode = "2440000A" }, ErrInvalidPostalCode},
		{"mcc", func(o *StaticOptions) { o.MCC = "58A2" }, ErrInvalidMCC},
	}
	for _, tt := range tests {
		o := ok
		tt.modify(&o)
		if p, err := NewStatic(o); !errors.Is(err, tt.want) {
			t.Errorf("%s: NewStatic = %q, %v; want %v", tt.name, p, err, tt.want)
		}
	}
	if _, err := NewStatic(ok); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	// Limits apply after transliteration: exactly 15 characters is fine.
	o := ok
	o.MerchantCity = "Niterói-RJ 2026"
	if _, err := NewStatic(o); err != nil {
		t.Errorf("15-character city: %v", err)
	}
}

func TestNewDynamic(t *testing.T) {
	p, err := NewDynamic(DynamicOptions{URL: "HTTPS://pix.example.com/qr/v2/9d36b84f", MerchantName: "Loja", MerchantCity: "Niterói"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Decode(p)
	if err != nil {
		t.Fatal(err)
	}
	if !b.Dynamic || b.Reusable || b.PointOfInitiation != "12" || b.URL != "pix.example.com/qr/v2/9d36b84f" || b.MerchantCity != "Niteroi" || b.TxID != "" {
		t.Errorf("Decode = %+v", b)
	}
	if issues := Lint(p); len(issues) != 0 {
		t.Errorf("Lint = %v", issues)
	}
	p, err = NewDynamic(DynamicOptions{URL: "pix.example.com/qr/1", MerchantName: "Loja", MerchantCity: "Rio", Reusable: true})
	if b, _ := Decode(p); err != nil || b.PointOfInitiation != "11" || !b.Reusable {
		t.Errorf("reusable: %q, %v", p, err)
	}

	ok := DynamicOptions{URL: "pix.example.com/qr/1", MerchantName: "Loja", MerchantCity: "Rio"}
	tests := []struct {
		name   string
		modify func(*DynamicOptions)
		want   error
	}{
		{"empty url", func(o *DynamicOptions) { o.URL = "https://" }, ErrInvalidURL},
		{"http", func(o *DynamicOptions) { o.URL = "http://pix.example.com/qr/1" }, ErrInvalidURL},
		{"space", func(o *DynamicOptions) { o.URL = "pix.example.com/q r" }, ErrInvalidURL},
		{"long", func(o *DynamicOptions) { o.URL = "pix.example.com/" + strings.Repeat("x", 70) }, ErrInvalidURL},
		{"name", func(o *DynamicOptions) { o.MerchantName = "" }, ErrInvalidMerchantName},
		{"city", func(o *DynamicOptions) { o.MerchantCity = "" }, ErrInvalidMerchantCity},
		{"mcc", func(o *DynamicOptions) { o.MCC = "1" }, ErrInvalidMCC},
	}
	for _, tt := range tests {
		o := ok
		tt.modify(&o)
		if p, err := NewDynamic(o); !errors.Is(err, tt.want) {
			t.Errorf("%s: NewDynamic = %q, %v; want %v", tt.name, p, err, tt.want)
		}
	}
}

func TestAmount(t *testing.T) {
	valid := map[string]int64{"0": 0, "1": 100, "10.5": 1050, "10.50": 1050, "0.01": 1, "007.10": 710, "9999999999.99": MaxCents}
	for in, want := range valid {
		if got, err := parseAmount(in); err != nil || got != want {
			t.Errorf("parseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	invalid := map[string]error{
		"": errAmountSyntax, ".5": errAmountSyntax, "10.": errAmountSyntax, "1.2.3": errAmountSyntax,
		"-1": errAmountSyntax, "1e3": errAmountSyntax, "10,50": errAmountComma, "1.234": errAmountDecimals,
		"12345678901234": errAmountTooLong,
	}
	for in, want := range invalid {
		if _, err := parseAmount(in); err != want {
			t.Errorf("parseAmount(%q) err = %v, want %v", in, err, want)
		}
	}
	for cents, want := range map[int64]string{0: "0.00", 1: "0.01", 10: "0.10", 1050: "10.50", MaxCents: "9999999999.99"} {
		if got := formatAmount(cents); got != want {
			t.Errorf("formatAmount(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestToASCII(t *testing.T) {
	tests := map[string]string{
		"São João":            "Sao Joao",
		"ÁÀÂÃÄ áàâãä":         "AAAAA aaaaa",
		"ÉÈÊË éèêë ÍÌÎÏ íìîï": "EEEE eeee IIII iiii",
		"ÓÒÔÕÖ óòôõö":         "OOOOO ooooo",
		"ÚÙÛÜ úùûü Çç Ññ":     "UUUU uuuu Cc Nn",
		"  plain  ":           "plain",
	}
	for in, want := range tests {
		if got, err := toASCII(in); err != nil || got != want {
			t.Errorf("toASCII(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"€", "a\x00b", "\xff"} {
		if got, err := toASCII(in); err == nil {
			t.Errorf("toASCII(%q) = %q, want error", in, got)
		}
	}
}

func FuzzNewStatic(f *testing.F) {
	f.Add("52998224725", "Fulano de Tal", "São Paulo", int64(1050), "PEDIDO1", "Café", "24400-000")
	f.Add("a@b.co", "X", "Y", int64(0), "", "", "")
	f.Fuzz(func(t *testing.T, key, name, city string, cents int64, txid, desc, postal string) {
		p, err := NewStatic(StaticOptions{Key: key, MerchantName: name, MerchantCity: city, Cents: cents, TxID: txid, Description: desc, PostalCode: postal})
		if err != nil {
			return
		}
		// Anything NewStatic produces must decode back and lint clean.
		b, err := Decode(p)
		if err != nil {
			t.Fatalf("Decode(%q): %v", p, err)
		}
		if b.Cents != cents {
			t.Fatalf("Cents = %d, want %d", b.Cents, cents)
		}
		if issues := Lint(p); len(issues) != 0 {
			t.Fatalf("Lint(%q) = %v", p, issues)
		}
	})
}
