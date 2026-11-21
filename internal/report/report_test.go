package report

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"529.982.247-25", `{"kind":"document","valid":true,"document":{"type":"cpf","number":"52998224725","formatted":"529.982.247-25","masked":"***.982.247-**","alphanumeric":false}}`},
		{"Fulano@Example.com", `{"kind":"pix_key","valid":true,"pix_key":{"type":"email","key":"fulano@example.com"}}`},
		{"E60701190202611211430abcdeFGHIJ1", `{"kind":"end_to_end_id","valid":true,"end_to_end_id":{"ispb":"60701190","time":"2026-11-21T14:30:00Z","sequence":"abcdeFGHIJ1"}}`},
		{"hello", `{"kind":"unknown","valid":false,"error":"brpay: unrecognized input"}`},
		{"00190.00009 00000.000000 00000.000075 9 10000000010000", `{"kind":"boleto","valid":true,"boleto":{"type":"bancario","barcode":"00199100000000100000000000000000000000000007","digitable_line":"00190000090000000000000000000075910000000010000","formatted":"00190.00009 00000.000000 00000.000075 9 10000000010000","amount_cents":10000,"bank_code":"001","bank_name":"Banco do Brasil","currency":9,"factor":1000,"due_date":"2025-02-22","free_field":"0000000000000000000000007"}}`},
		{"82650000000100000010000000000000000000000000", `{"kind":"boleto","valid":true,"boleto":{"type":"arrecadacao","barcode":"82650000000100000010000000000000000000000000","digitable_line":"826500000003100000010007000000000000000000000000","formatted":"82650000000-3 10000001000-7 00000000000-0 00000000000-0","amount_cents":1000,"free_field":"0000000000000000000000000","segment":2,"segment_name":"Saneamento","value_type":6,"modulus":10,"company_id":"0001"}}`},
	}
	for _, tt := range tests {
		b, err := json.Marshal(Decode(tt.in, now))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.want {
			t.Errorf("Decode(%q)\n got %s\nwant %s", tt.in, b, tt.want)
		}
	}
}

func TestDecodeDiagnostics(t *testing.T) {
	r := Decode("00190.00009 00000.500000 00000.000075 9 10000000010000", now)
	if r.Valid || r.Kind != "boleto" || len(r.Suggestions) != 2 || r.Suggestions[1].Formatted != "00190.00009 00000.000000 00000.000075 9 10000000010000" {
		t.Fatalf("%+v", r)
	}
	r = Decode("000201xx", now)
	if r.Valid || len(r.Issues) == 0 || r.Issues[0].Severity != "error" {
		t.Fatalf("%+v", r)
	}
}

func TestBuildStaticRoundTrip(t *testing.T) {
	s := BuildStatic(StaticOptions{Key: "+55 21 98765-4321", Name: "Padaria", City: "Niteroi", Amount: "12,50", TxID: "PEDIDO42"})
	if s.Error != "" {
		t.Fatal(s.Error)
	}
	r := Decode(s.Payload, now)
	if !r.Valid || r.Pix.Amount != "12.50" || r.Pix.KeyType != "phone" || r.Pix.TxID != "PEDIDO42" || r.Pix.Initiation != "static" {
		t.Fatalf("%+v %+v", r, r.Pix)
	}
	if s := BuildStatic(StaticOptions{Key: "x", Name: "A", City: "B"}); s.Error == "" || s.Payload != "" {
		t.Fatalf("%+v", s)
	}
	if s := BuildStatic(StaticOptions{Key: "a@b.co", Name: "A", City: "B", Amount: "1.234"}); !strings.Contains(s.Error, "amount") {
		t.Fatalf("%+v", s)
	}
}

func TestParseCents(t *testing.T) {
	for in, want := range map[string]int64{"": 0, "0": 0, "10": 1000, "10.5": 1050, "10.50": 1050, "10,50": 1050, " 7.01 ": 701, "999999999999.99": 99999999999999} {
		got, err := ParseCents(in)
		if err != nil || got != want {
			t.Errorf("ParseCents(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{".5", "10.", "10.505", "1.000,00", "-1", "1e3", "abc", "1234567890123", "10.5x"} {
		if _, err := ParseCents(in); !errors.Is(err, ErrAmount) {
			t.Errorf("ParseCents(%q) err = %v", in, err)
		}
	}
}
