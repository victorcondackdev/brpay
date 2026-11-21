package pix

import (
	"errors"
	"strings"
	"testing"

	"github.com/victorcondackdev/brpay/emv"
)

// bcbExample is the static BR Code from the BCB "Manual de Padrões para
// Iniciação do Pix". Its CRC is asserted independently in TestBCBExampleCRC.
const bcbExample = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
	"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

// withCRC appends the CRC so hand-written vectors stay valid.
func withCRC(body string) string { return emv.AppendCRC(body) }

// tlv encodes one data object, computing its length.
func tlv(id, value string) string {
	return id + string(rune('0'+len(value)/10)) + string(rune('0'+len(value)%10)) + value
}

func TestBCBExampleCRC(t *testing.T) {
	if got := emv.CRC16("123456789"); got != "29B1" {
		t.Fatalf("CRC16 check value = %s, want 29B1", got)
	}
	if got := emv.CRC16(bcbExample[:len(bcbExample)-4]); got != "1D3D" {
		t.Fatalf("CRC16(bcbExample) = %s, want 1D3D", got)
	}
}

func TestDecodeStatic(t *testing.T) {
	b, err := Decode("  " + bcbExample + "\r\n")
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"Static", b.Static, true},
		{"Dynamic", b.Dynamic, false},
		{"Reusable", b.Reusable, true},
		{"PointOfInitiation", b.PointOfInitiation, ""},
		{"MerchantAccountID", b.MerchantAccountID, "26"},
		{"Key", b.Key, "123e4567-e12b-12d1-a456-426655440000"},
		{"KeyType", b.KeyType, KeyEVP},
		{"MCC", b.MCC, "0000"},
		{"Currency", b.Currency, "986"},
		{"Amount", b.Amount, ""},
		{"Cents", b.Cents, int64(0)},
		{"CountryCode", b.CountryCode, "BR"},
		{"MerchantName", b.MerchantName, "Fulano de Tal"},
		{"MerchantCity", b.MerchantCity, "BRASILIA"},
		{"TxID", b.TxID, ""},
		{"CRC", b.CRC, "1D3D"},
		{"Templates", b.Templates["26"], GUI},
		{"Fields", len(b.Fields), 9},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %#v, want %#v", c.name, c.got, c.want)
		}
	}
	if f, ok := b.Template("62"); !ok || f.Children[0].Value != "***" {
		t.Errorf("Template(62) = %+v, %v", f, ok)
	}
}

func TestDecodeFull(t *testing.T) {
	body := "000201010211" +
		tlv("26", tlv("00", "BR.GOV.BCB.PIX")+tlv("01", "52998224725")+tlv("02", "Almoco!")) +
		tlv("27", tlv("00", "br.com.other")+tlv("01", "abcdefghij")) +
		"52045812" + "5303986" + "540510.50" + "5802BR" + tlv("59", "Joao Silva") + tlv("60", "Sao Paulo") + "610801311000" +
		"62140510PEDIDO1234" +
		tlv("80", tlv("00", "br.gov.bcb.future")+tlv("01", "data"))
	b, err := Decode(withCRC(body))
	if err != nil {
		t.Fatal(err)
	}
	if b.Key != "52998224725" || b.KeyType != KeyCPF || b.Description != "Almoco!" || b.Cents != 1050 ||
		b.Amount != "10.50" || b.TxID != "PEDIDO1234" || b.PostalCode != "01311000" || b.MCC != "5812" ||
		b.PointOfInitiation != "11" || !b.Reusable {
		t.Errorf("Decode = %+v", b)
	}
	if b.Templates["26"] != "BR.GOV.BCB.PIX" || b.Templates["27"] != "br.com.other" || b.Templates["80"] != "br.gov.bcb.future" {
		t.Errorf("Templates = %v", b.Templates)
	}
	if f, ok := b.Template("80"); !ok || f.Children[1].Value != "data" {
		t.Errorf("unreserved template lost: %+v", f)
	}
}

func TestDecodeDynamic(t *testing.T) {
	body := "000201010212" + "26420014br.gov.bcb.pix2520pix.example.com/qr/1" +
		"52040000" + "5303986" + "5802BR" + "5903Loj" + "6002RJ" + "62070503***"
	b, err := Decode(withCRC(body))
	if err != nil {
		t.Fatal(err)
	}
	if !b.Dynamic || b.Static || b.Reusable || b.URL != "pix.example.com/qr/1" || b.KeyType != KeyUnknown || b.Key != "" {
		t.Errorf("Decode = %+v", b)
	}
}

func TestDecodeErrors(t *testing.T) {
	const tail = "52040000" + "5303986" + "5802BR" + "5903Loj" + "6002RJ" + "62070503***"
	var acct = tlv("26", tlv("00", GUI)+tlv("01", "a@b.c.de"))
	tests := []struct {
		name    string
		payload string
		want    error
	}{
		{"syntax", "000201XX", emv.ErrSyntax},
		{"structure", withCRC("000202" + acct + tail), emv.ErrStructure},
		{"crc", bcbExample[:len(bcbExample)-4] + "0000", emv.ErrCRC},
		{"not pix", withCRC("000201" + tlv("26", tlv("00", "br.com.other")) + tail), ErrNotPix},
		{"key and url", withCRC("000201" + tlv("26", tlv("00", GUI)+tlv("01", "k")+tlv("25", "x.com/qr/1")) + tail), ErrKeyAndURL},
		{"neither", withCRC("000201" + tlv("26", tlv("00", GUI)+tlv("02", "whatever")) + tail), ErrNoKeyOrURL},
		{"amount", withCRC("000201" + acct + "540510,50" + tail), ErrInvalidAmount},
		{"missing 59", withCRC("000201" + acct + strings.Replace(tail, "5903Loj", "", 1)), ErrMissingField},
		{"missing 52", withCRC("000201" + acct + strings.Replace(tail, "52040000", "", 1)), ErrMissingField},
	}
	for _, tt := range tests {
		b, err := Decode(tt.payload)
		if !errors.Is(err, tt.want) || b != nil {
			t.Errorf("%s: Decode = %v, %v; want %v", tt.name, b, err, tt.want)
		}
	}
	_, err := Decode(withCRC("000201" + acct + strings.Replace(tail, "5903Loj", "", 1)))
	if err.Error() != "pix: missing required field: 59 (merchant name)" {
		t.Errorf("message = %q", err)
	}
}

func FuzzDecode(f *testing.F) {
	seeds := []string{
		bcbExample,
		withCRC("000201010212" + "26420014br.gov.bcb.pix2520pix.example.com/qr/1" + "52040000530398654041.005802BR5903Loj6002RJ62070503***"),
		"",
		"000201",
		"00020126180014br.gov.bcb.pix6304ABCD",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		b, err := Decode(s)
		issues := Lint(s)
		if err != nil {
			return
		}
		if b.Static == b.Dynamic {
			t.Fatalf("Decode(%q): Static=%v Dynamic=%v", s, b.Static, b.Dynamic)
		}
		if b.Cents < 0 || b.Cents > MaxCents {
			t.Fatalf("Decode(%q): Cents=%d", s, b.Cents)
		}
		for _, i := range issues {
			if i.Field == "63" && i.Severity == SeverityError {
				t.Fatalf("Decode accepted %q but Lint reports a CRC error: %v", s, i)
			}
		}
	})
}

func BenchmarkDecode(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(bcbExample); err != nil {
			b.Fatal(err)
		}
	}
}
