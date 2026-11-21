package document

import "testing"

func TestCPF(t *testing.T) {
	valid := []string{"529.982.247-25", "52998224725", "111.444.777-35"}
	for _, v := range valid {
		if err := ValidateCPF(v); err != nil {
			t.Errorf("ValidateCPF(%q) = %v", v, err)
		}
	}
	invalid := map[string]error{
		"529.982.247-24": ErrCheckDigit,
		"111.111.111-11": ErrRepeated,
		"1234567890":     ErrLength,
		"5299822472A":    ErrCharacters,
	}
	for v, want := range invalid {
		if err := ValidateCPF(v); err != want {
			t.Errorf("ValidateCPF(%q) = %v, want %v", v, err, want)
		}
	}
}

func TestCNPJNumeric(t *testing.T) {
	for _, v := range []string{"11.222.333/0001-81", "11222333000181"} {
		if err := ValidateCNPJ(v); err != nil {
			t.Errorf("ValidateCNPJ(%q) = %v", v, err)
		}
	}
	if err := ValidateCNPJ("11.222.333/0001-80"); err != ErrCheckDigit {
		t.Errorf("want ErrCheckDigit, got %v", err)
	}
}

// Example published by the Receita Federal for the alphanumeric CNPJ.
func TestCNPJAlphanumeric(t *testing.T) {
	const v = "12.ABC.345/01DE-35"
	if err := ValidateCNPJ(v); err != nil {
		t.Fatalf("ValidateCNPJ(%q) = %v", v, err)
	}
	if !IsAlphanumericCNPJ(v) {
		t.Error("expected alphanumeric")
	}
	if err := ValidateCNPJ("12.abc.345/01de-35"); err != nil {
		t.Errorf("lowercase should be accepted: %v", err)
	}
	if err := ValidateCNPJ("12ABC34501DE3A"); err != ErrCharacters {
		t.Errorf("letter in check digit: got %v", err)
	}
}

func TestDetectFormatMask(t *testing.T) {
	if Detect("52998224725") != CPF || Detect("11222333000181") != CNPJ || Detect("123") != Unknown {
		t.Fatal("Detect mismatch")
	}
	if got := Format("52998224725"); got != "529.982.247-25" {
		t.Errorf("Format CPF = %q", got)
	}
	if got := Format("12abc34501de35"); got != "12.ABC.345/01DE-35" {
		t.Errorf("Format CNPJ = %q", got)
	}
	if got := Mask("52998224725"); got != "***.982.247-**" {
		t.Errorf("Mask CPF = %q", got)
	}
}

func FuzzValidate(f *testing.F) {
	f.Add("529.982.247-25")
	f.Add("12.ABC.345/01DE-35")
	f.Fuzz(func(t *testing.T, s string) {
		_ = ValidateCPF(s)
		_ = ValidateCNPJ(s)
		_ = Format(s)
		_ = Mask(s)
	})
}
