package emv

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// bcbExample is the static BR Code published in the Banco Central do Brasil
// "Manual de Padrões para Iniciação do Pix".
const bcbExample = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-426655440000" +
	"5204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

// crcBitwise is an independent, table-free CRC-16/CCITT-FALSE used to check
// the table-driven implementation.
func crcBitwise(s string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func TestCRC16(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"123456789", "29B1"}, // standard CRC-16/CCITT-FALSE check value
		{"", "FFFF"},
		{bcbExample[:len(bcbExample)-4], "1D3D"},
	}
	for _, tt := range tests {
		if got := CRC16(tt.in); got != tt.want {
			t.Errorf("CRC16(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
	for _, s := range []string{"a", "hello world", bcbExample, strings.Repeat("\xff", 300)} {
		if got, want := Checksum(s), crcBitwise(s); got != want {
			t.Errorf("Checksum(%q) = %04X, bitwise = %04X", s, got, want)
		}
	}
}

func TestAppendVerifyCRC(t *testing.T) {
	p := AppendCRC(bcbExample[:len(bcbExample)-8])
	if p != bcbExample {
		t.Fatalf("AppendCRC = %q", p)
	}
	if err := VerifyCRC(bcbExample); err != nil {
		t.Fatalf("VerifyCRC: %v", err)
	}
	if err := VerifyCRC(bcbExample[:len(bcbExample)-4] + "1d3d"); err != nil {
		t.Errorf("lowercase hex should verify: %v", err)
	}

	tests := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"wrong", bcbExample[:len(bcbExample)-4] + "1D3E", ErrCRC},
		{"not hex", bcbExample[:len(bcbExample)-4] + "ZZZZ", ErrCRC},
		{"missing", bcbExample[:len(bcbExample)-8], ErrStructure},
		{"short", "63", ErrStructure},
		{"empty", "", ErrStructure},
	}
	for _, tt := range tests {
		err := VerifyCRC(tt.in)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: VerifyCRC = %v, want %v", tt.name, err, tt.wantErr)
		}
	}
	var ce *CRCError
	if err := VerifyCRC(bcbExample[:len(bcbExample)-4] + "0000"); !errors.As(err, &ce) || ce.Want != "1D3D" || ce.Got != "0000" {
		t.Errorf("CRCError = %#v", ce)
	}
	if got := (&CRCError{Got: "0000", Want: "1D3D"}).Error(); got != `emv: CRC mismatch: payload has "0000", computed 1D3D` {
		t.Errorf("Error() = %q", got)
	}
}

func TestParse(t *testing.T) {
	fields, err := Parse(bcbExample)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(fields))
	for i, f := range fields {
		ids[i] = f.ID
	}
	if want := []string{"00", "26", "52", "53", "58", "59", "60", "62", "63"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	acct, ok := Find(fields, "26")
	if !ok || len(acct.Children) != 2 {
		t.Fatalf("26 = %+v", acct)
	}
	if key, _ := Find(acct.Children, "01"); key.Value != "123e4567-e12b-12d1-a456-426655440000" {
		t.Errorf("key = %q", key.Value)
	}
	if name, _ := Find(fields, "59"); name.Value != "Fulano de Tal" || name.Children != nil {
		t.Errorf("59 = %+v", name)
	}
	if _, ok := Find(fields, "54"); ok {
		t.Error("54 should be absent")
	}
	if err := Validate(fields); err != nil {
		t.Errorf("Validate: %v", err)
	}
	got, err := Encode(fields)
	if err != nil || got != bcbExample {
		t.Errorf("Encode round trip = %q, %v", got, err)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in     string
		offset int
		id     string
		reason string
	}{
		{"000201AB01x", 6, "", "invalid field ID \"AB\": want 2 digits"},
		{"0002010", 6, "", "truncated field header: need 4 bytes, have 1"},
		{"00020159XX", 8, "59", "invalid length \"XX\": want 2 digits"},
		{"00020159300123456789AB", 6, "59", "length 30 exceeds remaining 12"},
		{"0000", 2, "00", "zero length"},
		{"000201620505031", 10, "62.05", "length 3 exceeds remaining 1"},
		{"00020162020X", 10, "62", "truncated field header: need 4 bytes, have 2"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("Parse(%q) err = %v, want *ParseError", tt.in, err)
			continue
		}
		if pe.Offset != tt.offset || pe.ID != tt.id || pe.Reason != tt.reason || !errors.Is(err, ErrSyntax) {
			t.Errorf("Parse(%q) = {%d %q %q}, want {%d %q %q}", tt.in, pe.Offset, pe.ID, pe.Reason, tt.offset, tt.id, tt.reason)
		}
	}
	want := "emv: byte 6, field 59: length 30 exceeds remaining 12"
	if _, err := Parse("00020159300123456789AB"); err == nil || err.Error() != want {
		t.Errorf("Error() = %v, want %q", err, want)
	}
	if got := (&ParseError{Offset: 3, Reason: "x"}).Error(); got != "emv: byte 3: x" {
		t.Errorf("Error() without ID = %q", got)
	}
}

func TestParseTemplate(t *testing.T) {
	got, err := ParseTemplate("0014br.gov.bcb.pix0104abcd")
	want := []Field{{ID: "00", Value: "br.gov.bcb.pix"}, {ID: "01", Value: "abcd"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ParseTemplate = %+v, %v", got, err)
	}
	// Templates are not expanded by ParseTemplate.
	got, err = ParseTemplate("62070503***")
	if err != nil || got[0].Children != nil {
		t.Errorf("ParseTemplate nested = %+v, %v", got, err)
	}
	if _, err := ParseTemplate("01"); err == nil {
		t.Error("want error")
	}
}

func TestIsTemplateID(t *testing.T) {
	tests := map[string]bool{
		"00": false, "25": false, "26": true, "51": true, "52": false, "62": true,
		"63": false, "64": true, "65": false, "79": false, "80": true, "99": true,
		"": false, "1": false, "ab": false, "100": false,
	}
	for id, want := range tests {
		if got := IsTemplateID(id); got != want {
			t.Errorf("IsTemplateID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestEncode(t *testing.T) {
	fields := []Field{
		{ID: "00", Value: "01"},
		{ID: "26", Children: []Field{{ID: "00", Value: "br.gov.bcb.pix"}, {ID: "01", Value: "k"}}},
		{ID: "62", Value: "ignored", Children: []Field{{ID: "05", Value: "***"}}},
		{ID: "59", Value: strings.Repeat("x", 99)},
	}
	got, err := Encode(fields)
	want := "000201" + "26230014br.gov.bcb.pix0101k" + "62070503***" + "5999" + strings.Repeat("x", 99)
	if err != nil || got != want {
		t.Fatalf("Encode = %q, %v\nwant %q", got, err, want)
	}

	bad := []struct {
		name   string
		fields []Field
		msg    string
	}{
		{"bad id", []Field{{ID: "5", Value: "x"}}, "emv: cannot encode field 5: ID must be 2 digits"},
		{"empty", []Field{{ID: "59"}}, "emv: cannot encode field 59: empty value"},
		{"too long", []Field{{ID: "59", Value: strings.Repeat("x", 100)}}, "emv: cannot encode field 59: value is 100 bytes, max 99"},
		{"nested", []Field{{ID: "62", Children: []Field{{ID: "05"}}}}, "emv: cannot encode field 62.05: empty value"},
		{"template too long", []Field{{ID: "26", Children: []Field{{ID: "01", Value: strings.Repeat("x", 96)}}}}, "emv: cannot encode field 26: value is 100 bytes, max 99"},
	}
	for _, tt := range bad {
		_, err := Encode(tt.fields)
		if !errors.Is(err, ErrEncode) || err.Error() != tt.msg {
			t.Errorf("%s: Encode err = %v, want %q", tt.name, err, tt.msg)
		}
	}
	if got, err := Encode(nil); got != "" || err != nil {
		t.Errorf("Encode(nil) = %q, %v", got, err)
	}
}

func TestValidate(t *testing.T) {
	f := func(pairs ...string) []Field {
		var out []Field
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, Field{ID: pairs[i], Value: pairs[i+1]})
		}
		return out
	}
	tests := []struct {
		name   string
		fields []Field
		offset int
		reason string
	}{
		{"ok", f("00", "01", "59", "X", "63", "ABCD"), 0, ""},
		{"empty", nil, 0, "empty payload"},
		{"00 not first", f("59", "X", "00", "01", "63", "ABCD"), 0, "payload format indicator (00) must be the first field"},
		{"00 value", f("00", "02", "63", "ABCD"), 0, `payload format indicator must be "01", got "02"`},
		{"63 not last", f("00", "01", "63", "ABCD", "59", "X"), 6, "CRC (63) must be the last field"},
		{"no 63", f("00", "01", "59", "X"), 6, "CRC (63) must be the last field"},
		{"63 length", f("00", "01", "63", "ABC"), 6, "CRC must be 4 characters, got 3"},
		{"duplicate", f("00", "01", "59", "X", "59", "Y", "63", "ABCD"), 11, "duplicate field"},
		{"bad id", f("00", "01", "x", "X", "63", "ABCD"), 6, "invalid field ID"},
		{"offset after template", []Field{{ID: "00", Value: "01"}, {ID: "62", Children: []Field{{ID: "05", Value: "***"}}}, {ID: "59", Value: "X"}}, 17, "CRC (63) must be the last field"},
	}
	for _, tt := range tests {
		err := Validate(tt.fields)
		if tt.reason == "" {
			if err != nil {
				t.Errorf("%s: Validate = %v", tt.name, err)
			}
			continue
		}
		var pe *ParseError
		if !errors.As(err, &pe) || pe.Reason != tt.reason || pe.Offset != tt.offset || !errors.Is(err, ErrStructure) {
			t.Errorf("%s: Validate = %v (offset %v), want offset %d %q", tt.name, err, pe, tt.offset, tt.reason)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{bcbExample, "", "0000", "000201", "000201620505031", "26580014br.gov.bcb.pix"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fields, err := Parse(s)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Offset < 0 || pe.Offset > len(s) {
				t.Fatalf("Parse(%q): bad error %v", s, err)
			}
			return
		}
		// A successful parse must re-encode to the identical input.
		got, err := Encode(fields)
		if err != nil || got != s {
			t.Fatalf("round trip: Parse(%q) -> Encode = %q, %v", s, got, err)
		}
		_ = Validate(fields)
		_ = VerifyCRC(s)
	})
}

func BenchmarkChecksum(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Checksum(bcbExample)
	}
}

func BenchmarkVerifyCRC(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := VerifyCRC(bcbExample); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(bcbExample); err != nil {
			b.Fatal(err)
		}
	}
}
