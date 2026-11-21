package boleto

import "testing"

func TestMod10(t *testing.T) {
	tests := []struct {
		in   string
		want byte
	}{
		{"0", '0'},
		{"5", '9'},          // 5×2 = 10 → 1+0 = 1 → (10−1)%10 = 9
		{"7992739871", '3'}, // classic Luhn example, same weighting
		// 3 4 1 9 1 0 9 0 1 with weights (right to left) 2 1 2 1 2 1 2 1 2:
		// 2 + 0 + (18→9) + 0 + 2 + 9 + 2 + 4 + 6 = 34 → (10−4)%10 = 6
		{"341910901", '6'},
		{"0000000000", '0'},
	}
	for _, tt := range tests {
		if got := mod10(tt.in); got != tt.want {
			t.Errorf("mod10(%q) = %c, want %c", tt.in, got, tt.want)
		}
	}
}

func TestMod11Bancario(t *testing.T) {
	tests := []struct {
		in   string
		want byte
	}{
		{"0", '1'},  // 11 − 0 = 11 → 1
		{"5", '1'},  // 10 % 11 = 10 → 11 − 10 = 1
		{"6", '1'},  // 12 % 11 = 1 → 10 → 1
		{"1", '9'},  // 2 → 9
		{"10", '8'}, // 1×3 = 3 → 8
	}
	for _, tt := range tests {
		if got := mod11Bancario(tt.in); got != tt.want {
			t.Errorf("mod11Bancario(%q) = %c, want %c", tt.in, got, tt.want)
		}
	}
}

func TestMod11Arrecadacao(t *testing.T) {
	tests := []struct {
		in   string
		want byte
	}{
		{"0", '0'},  // remainder 0 → 0
		{"6", '0'},  // 12 % 11 = 1 → 0
		{"5", '1'},  // 10 % 11 = 10 → 1
		{"1", '9'},  // 2 → 9
		{"10", '8'}, // 3 → 8
		// weights cycle back to 2 after 9: "100000000" → 1×2 = 2 → 9
		{"100000000", '9'},
	}
	for _, tt := range tests {
		if got := mod11Arrecadacao(tt.in); got != tt.want {
			t.Errorf("mod11Arrecadacao(%q) = %c, want %c", tt.in, got, tt.want)
		}
	}
}
