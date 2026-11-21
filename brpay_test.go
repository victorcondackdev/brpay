package brpay

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/victorcondackdev/brpay/boleto"
	"github.com/victorcondackdev/brpay/document"
	"github.com/victorcondackdev/brpay/emv"
	"github.com/victorcondackdev/brpay/pix"
)

const (
	staticPix = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
		"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"
	bbLine      = "00190.00009 00000.000000 00000.000075 9 10000000010000"
	bbBarcode   = "00199100000000100000000000000000000000000007"
	bbLineTypo  = "00190.00009 00000.500000 00000.000075 9 10000000010000"
	arrLine     = "82650000000-3 10000001000-7 00000000000-0 00000000000-0"
	arrBarcode  = "82650000000100000010000000000000000000000000"
	e2e         = "E60701190202611211430abcdeFGHIJ1"
	evp         = "123e4567-e12b-12d1-a456-426655440000"
	alnumCNPJ   = "12.ABC.345/01DE-35"
	numericCNPJ = "11.222.333/0001-81"
	cpf         = "529.982.247-25"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Kind
	}{
		{"empty", "", KindUnknown},
		{"blank", " \n\t", KindUnknown},
		{"pix", staticPix, KindPix},
		{"pix with newline", staticPix + "\n", KindPix},
		{"pix broken", "000201garbage", KindPix},
		{"boleto line formatted", bbLine, KindBoleto},
		{"boleto barcode", bbBarcode, KindBoleto},
		{"boleto line typo", bbLineTypo, KindBoleto},
		{"arrecadação line", arrLine, KindBoleto},
		{"arrecadação barcode", arrBarcode, KindBoleto},
		{"43 digits", bbBarcode[:43], KindUnknown},
		{"cpf formatted", cpf, KindDocument},
		{"cpf bare", "52998224725", KindDocument},
		{"cpf bad check digit", "52998224724", KindDocument},
		{"cnpj numeric", numericCNPJ, KindDocument},
		{"cnpj alphanumeric", alnumCNPJ, KindDocument},
		{"cnpj alphanumeric lowercase", strings.ToLower(alnumCNPJ), KindDocument},
		{"end to end id", e2e, KindEndToEndID},
		{"email", "fulano@example.com", KindPixKey},
		{"phone", "+55 (21) 98765-4321", KindPixKey},
		{"evp", evp, KindPixKey},
		{"evp uppercase", strings.ToUpper(evp), KindPixKey},
		{"word", "hello", KindUnknown},
		{"12 digits", "123456789012", KindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.input); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// A linha digitável of bank "000", currency 2, starts with "000201", the Pix
// payload format indicator. A valid boleto must win.
func TestDetectValidBoletoWinsOverPixPrefix(t *testing.T) {
	var line string
	for dac := 0; dac < 10 && line == ""; dac++ {
		code := fmt.Sprintf("0002%d1000%010d01%023d", dac, 0, 0)
		if boleto.Validate(code) == nil {
			line, _ = boleto.BarcodeToLine(code)
		}
	}
	if !strings.HasPrefix(line, "000201") {
		t.Fatalf("bad fixture %q", line)
	}
	if got := Detect(line); got != KindBoleto {
		t.Fatalf("Detect(%q) = %v, want boleto", line, got)
	}
	// With a broken check digit it is no longer a valid boleto, and the
	// Pix prefix takes over.
	broken := line[:46] + string(rune('0'+(line[46]-'0'+1)%10))
	if got := Detect(broken); got != KindPix {
		t.Fatalf("Detect(%q) = %v, want pix", broken, got)
	}
}

func TestDecodePix(t *testing.T) {
	r, err := Decode(staticPix)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != KindPix || r.Pix == nil || r.Pix.Key != evp || len(r.Issues) != 0 {
		t.Fatalf("got %+v", r)
	}
	if r.Boleto != nil || r.Document != nil || r.PixKey != nil || r.EndToEndID != nil {
		t.Fatalf("more than one payload set: %+v", r)
	}
}

func TestDecodePixLintsEvenOnFailure(t *testing.T) {
	broken := strings.Replace(staticPix, "63041D3D", "6304FFFF", 1)
	r, err := Decode(broken)
	if !errors.Is(err, emv.ErrCRC) {
		t.Fatalf("err = %v, want emv.ErrCRC", err)
	}
	if r.Kind != KindPix || r.Pix != nil {
		t.Fatalf("got %+v", r)
	}
	if len(r.Issues) == 0 || r.Issues[0].Field != "63" || r.Issues[0].Severity != pix.SeverityError {
		t.Fatalf("issues = %v, want a CRC error", r.Issues)
	}

	var pe *emv.ParseError
	r, err = Decode("000201269")
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v, want *emv.ParseError", err)
	}
	if len(r.Issues) == 0 {
		t.Fatal("no issues for a malformed payload")
	}
}

func TestDecodePixWhitespaceWarning(t *testing.T) {
	r, err := Decode(staticPix + "\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pix == nil || len(r.Issues) != 1 || r.Issues[0].Severity != pix.SeverityWarning {
		t.Fatalf("got pix=%v issues=%v", r.Pix != nil, r.Issues)
	}
}

func TestDecodeBoleto(t *testing.T) {
	for _, in := range []string{bbLine, bbBarcode, arrLine, arrBarcode} {
		r, err := Decode(in)
		if err != nil {
			t.Fatalf("Decode(%q): %v", in, err)
		}
		if r.Kind != KindBoleto || r.Boleto == nil || r.Suggestions != nil {
			t.Fatalf("Decode(%q) = %+v", in, r)
		}
	}
}

func TestDecodeBoletoSuggestions(t *testing.T) {
	r, err := Decode(bbLineTypo)
	var cde *boleto.CheckDigitError
	if !errors.As(err, &cde) || cde.Field != 2 {
		t.Fatalf("err = %v, want check digit error in field 2", err)
	}
	if r.Boleto != nil {
		t.Fatal("Boleto set on error")
	}
	want := strings.NewReplacer(".", "", " ", "").Replace(bbLine)
	found := false
	for _, s := range r.Suggestions {
		found = found || s == want
	}
	if !found {
		t.Fatalf("suggestions %v do not include %s", r.Suggestions, want)
	}

	// A barcode has no per-field check digits to locate a typo with.
	r, err = Decode("00199100000000100000000000000000000000000008")
	if !errors.Is(err, boleto.ErrCheckDigit) || r.Suggestions != nil {
		t.Fatalf("err=%v suggestions=%v", err, r.Suggestions)
	}
}

func TestDecodeDocument(t *testing.T) {
	tests := []struct {
		in        string
		kind      document.Kind
		formatted string
		alnum     bool
	}{
		{cpf, document.CPF, cpf, false},
		{"52998224725", document.CPF, cpf, false},
		{numericCNPJ, document.CNPJ, numericCNPJ, false},
		{"12abc34501de35", document.CNPJ, alnumCNPJ, true},
	}
	for _, tt := range tests {
		r, err := Decode(tt.in)
		if err != nil {
			t.Fatalf("Decode(%q): %v", tt.in, err)
		}
		d := r.Document
		if r.Kind != KindDocument || d == nil || d.Kind != tt.kind || d.Formatted != tt.formatted ||
			d.Alphanumeric != tt.alnum || d.Masked == "" || strings.ContainsAny(d.Number, ".-/") {
			t.Fatalf("Decode(%q) = %+v", tt.in, d)
		}
	}
	for _, in := range []string{"52998224724", "111.111.111-11", "11.222.333/0001-82"} {
		r, err := Decode(in)
		if err == nil || r.Kind != KindDocument || r.Document != nil {
			t.Fatalf("Decode(%q) = %+v, %v; want document error", in, r, err)
		}
	}
	if _, err := Decode("52998224724"); !errors.Is(err, document.ErrCheckDigit) {
		t.Fatalf("err = %v, want document.ErrCheckDigit", err)
	}
}

func TestDecodePixKeyAndEndToEndID(t *testing.T) {
	r, err := Decode("+55 (21) 98765-4321")
	if err != nil || r.PixKey == nil || r.PixKey.Type != pix.KeyPhone || r.PixKey.Key != "+5521987654321" {
		t.Fatalf("phone: %+v %v", r.PixKey, err)
	}
	r, err = Decode(strings.ToUpper(evp))
	if err != nil || r.PixKey == nil || r.PixKey.Type != pix.KeyEVP || r.PixKey.Key != evp {
		t.Fatalf("evp: %+v %v", r.PixKey, err)
	}
	if r, err = Decode("not-an-email@"); !errors.Is(err, pix.ErrInvalidKey) || r.Kind != KindPixKey {
		t.Fatalf("bad email: %v %v", r.Kind, err)
	}
	r, err = Decode(e2e)
	if err != nil || r.EndToEndID == nil || r.EndToEndID.ISPB != "60701190" {
		t.Fatalf("e2e: %+v %v", r.EndToEndID, err)
	}
	if _, err = Decode("E60701190202613211430abcdeFGHIJ1"); !errors.Is(err, pix.ErrInvalidEndToEndID) {
		t.Fatalf("bad e2e: %v", err)
	}
}

func TestDecodeUnrecognized(t *testing.T) {
	for _, in := range []string{"", "hello", "123"} {
		r, err := Decode(in)
		if !errors.Is(err, ErrUnrecognized) || r == nil || r.Kind != KindUnknown {
			t.Fatalf("Decode(%q) = %+v, %v", in, r, err)
		}
	}
}

func TestKindString(t *testing.T) {
	for k, want := range map[Kind]string{
		KindUnknown: "unknown", KindPix: "pix", KindBoleto: "boleto", KindDocument: "document",
		KindPixKey: "pix_key", KindEndToEndID: "end_to_end_id", Kind(-1): "unknown", Kind(99): "unknown",
	} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, want)
		}
		if b, _ := k.MarshalText(); string(b) != want {
			t.Errorf("Kind(%d).MarshalText() = %q", int(k), b)
		}
	}
}

func FuzzDecode(f *testing.F) {
	for _, s := range []string{staticPix, bbLine, bbBarcode, bbLineTypo, arrLine, e2e, evp, cpf, alnumCNPJ,
		"fulano@example.com", "+5521987654321", "000201", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := Decode(s)
		if r == nil {
			t.Fatal("nil result")
		}
		if r.Kind != Detect(s) {
			t.Fatalf("Decode kind %v != Detect %v", r.Kind, Detect(s))
		}
		set := 0
		for _, ok := range []bool{r.Pix != nil, r.Boleto != nil, r.Document != nil, r.PixKey != nil, r.EndToEndID != nil} {
			if ok {
				set++
			}
		}
		if err == nil && set != 1 || err != nil && set != 0 {
			t.Fatalf("err=%v with %d payloads set", err, set)
		}
		if r.Kind != KindPix && r.Issues != nil || r.Kind != KindBoleto && r.Suggestions != nil {
			t.Fatalf("diagnostics for kind %v", r.Kind)
		}
		if err == nil && r.Kind == KindBoleto {
			// A decoded boleto must decode the same way from every form.
			for _, in := range []string{r.Boleto.Barcode, r.Boleto.DigitableLine, r.Boleto.Formatted()} {
				if r2, err := Decode(in); err != nil || r2.Kind != KindBoleto {
					t.Fatalf("Decode(%q) = %v, %v", in, r2.Kind, err)
				}
			}
		}
		if err == nil && r.Document != nil && !utf8.ValidString(r.Document.Formatted) {
			t.Fatal("invalid UTF-8")
		}
	})
}
