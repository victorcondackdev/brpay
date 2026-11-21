package pix

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
		typ  KeyType
	}{
		{"529.982.247-25", "52998224725", KeyCPF},
		{" 52998224725 ", "52998224725", KeyCPF},
		{"11.222.333/0001-81", "11222333000181", KeyCNPJ},
		{"12.abc.345/01de-35", "12ABC34501DE35", KeyCNPJ},
		{"Fulano.Tal@Example.COM", "fulano.tal@example.com", KeyEmail},
		{"a+tag@sub.example.com.br", "a+tag@sub.example.com.br", KeyEmail},
		{"+55 (21) 98765-4321", "+5521987654321", KeyPhone},
		{"+552133334444", "+552133334444", KeyPhone},
		{"123E4567-E12B-12D1-A456-426655440000", "123e4567-e12b-12d1-a456-426655440000", KeyEVP},
	}
	for _, tt := range tests {
		got, typ, err := NormalizeKey(tt.in)
		if err != nil || got != tt.want || typ != tt.typ {
			t.Errorf("NormalizeKey(%q) = %q, %v, %v; want %q, %v", tt.in, got, typ, err, tt.want, tt.typ)
		}
		if typ, err := DetectKey(tt.in); err != nil || typ != tt.typ {
			t.Errorf("DetectKey(%q) = %v, %v", tt.in, typ, err)
		}
	}
}

func TestNormalizeKeyInvalid(t *testing.T) {
	tests := []struct {
		in  string
		msg string
	}{
		{"", "empty"},
		{"   ", "empty"},
		{"529.982.247-24", "CPF"},
		{"21987654321", "phone keys must start with +55"},
		{"11.222.333/0001-80", "CNPJ"},
		{"12345", "not a CPF"},
		{"a@b@example.com", "more than one @"},
		{"@example.com", "local part"},
		{strings.Repeat("a", 65) + "@example.com", "local part"},
		{strings.Repeat("a", 70) + "@example.com", "longer than 77"},
		{".a@example.com", "misplaced dot"},
		{"a..b@example.com", "misplaced dot"},
		{"a.@example.com", "misplaced dot"},
		{"a b@example.com", "invalid character"},
		{"joão@example.com", "invalid character"},
		{"a@localhost", "must contain a dot"},
		{"a@-example.com", "invalid e-mail domain"},
		{"a@example..com", "invalid e-mail domain"},
		{"a@exa_mple.com", "invalid character"},
		{"+1 212 555 0100", "Brazilian"},
		{"+", "Brazilian"},
		{"+55 21 9876-543", "area code and 8 or 9"},
		{"+55 21 98765-43210", "area code and 8 or 9"},
		{"+55 01 98765-4321", "area code"},
		{"+55 21 88765-4321", "must start with 9"},
		{"+55 21 98765#4321", "invalid character"},
		{"+55123456789012345678", "too long"},
		{"123e4567-e12b-12d1-a456-42665544000g", "hexadecimal"},
	}
	for _, tt := range tests {
		got, typ, err := NormalizeKey(tt.in)
		if !errors.Is(err, ErrInvalidKey) || !strings.Contains(err.Error(), tt.msg) || got != "" || typ != KeyUnknown {
			t.Errorf("NormalizeKey(%q) = %q, %v, %v; want error containing %q", tt.in, got, typ, err, tt.msg)
		}
	}
}

func TestKeyTypeString(t *testing.T) {
	want := map[KeyType]string{KeyUnknown: "UNKNOWN", KeyCPF: "CPF", KeyCNPJ: "CNPJ", KeyEmail: "EMAIL", KeyPhone: "PHONE", KeyEVP: "EVP", KeyType(99): "UNKNOWN"}
	for k, s := range want {
		if k.String() != s {
			t.Errorf("%d.String() = %q, want %q", k, k.String(), s)
		}
	}
}

func FuzzNormalizeKey(f *testing.F) {
	for _, s := range []string{"529.982.247-25", "12.ABC.345/01DE-35", "A@B.CO", "+55 21 98765-4321", "123E4567-E12B-12D1-A456-426655440000", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		k, typ, err := NormalizeKey(s)
		if err != nil {
			return
		}
		// Normalization is idempotent and preserves the type.
		k2, typ2, err := NormalizeKey(k)
		if err != nil || k2 != k || typ2 != typ {
			t.Fatalf("NormalizeKey(%q) = %q %v; again = %q %v %v", s, k, typ, k2, typ2, err)
		}
	})
}
