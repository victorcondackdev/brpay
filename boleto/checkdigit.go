package boleto

// mod10 returns the FEBRABAN modulo-10 check digit of an ASCII digit string:
// weights 2, 1, 2, 1… from right to left, the digits of each product are
// summed, and the check digit is (10 − sum mod 10) mod 10.
func mod10(digits string) byte {
	sum, w := 0, 2
	for i := len(digits) - 1; i >= 0; i-- {
		p := int(digits[i]-'0') * w
		sum += p/10 + p%10
		w = 3 - w
	}
	return byte('0' + (10-sum%10)%10)
}

// sum11 returns the weighted sum used by both modulo-11 variants: weights
// 2 to 9, cycling, from right to left.
func sum11(digits string) int {
	sum, w := 0, 2
	for i := len(digits) - 1; i >= 0; i-- {
		sum += int(digits[i]-'0') * w
		w++
		if w > 9 {
			w = 2
		}
	}
	return sum
}

// mod11Bancario returns the bancário general check digit (DAC):
// 11 − (sum mod 11), with results 0, 10 and 11 replaced by 1.
func mod11Bancario(digits string) byte {
	dv := 11 - sum11(digits)%11
	if dv == 0 || dv > 9 {
		dv = 1
	}
	return byte('0' + dv)
}

// mod11Arrecadacao returns the arrecadação modulo-11 check digit: remainder
// 0 or 1 gives 0, remainder 10 gives 1, otherwise 11 − remainder.
func mod11Arrecadacao(digits string) byte {
	switch r := sum11(digits) % 11; r {
	case 0, 1:
		return '0'
	case 10:
		return '1'
	default:
		return byte('0' + 11 - r)
	}
}

// isDigits reports whether s contains only ASCII digits.
func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
