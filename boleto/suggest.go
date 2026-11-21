package boleto

// MaxSuggestions caps the number of candidates returned by [SuggestFix].
// It is above the worst case for a single typo (43 candidates, for a
// bancário typo in field 5), so the correct line is never cut off.
const MaxSuggestions = 50

// SuggestFix proposes corrections for a linha digitável (47 or 48 digits,
// punctuation allowed) that contains exactly one mistyped digit.
//
// It first locates the typo using the per-field check digits, then tries
// the nine other values at every plausible position and keeps the
// candidates that make the whole line valid, including the general check
// digit:
//
//   - one field fails its own check digit: every digit of that field (for
//     arrecadação, also the value-type digit, which selects the modulus);
//   - bancário, no field fails: fields 4 and 5, since field 5 has no check
//     digit of its own and modulo 10 catches any single typo in fields 1–3;
//   - arrecadação, several blocks fail or the value type is invalid: the
//     value-type digit, the only single typo that breaks several blocks;
//   - bancário, several fields fail: more than one typo, no suggestion.
//
// A single typo usually yields one to a few candidates; check digits cannot
// always tell them apart (e.g. modulo-10 blocks whose weights align with
// the general check digit), so present them to the user to choose from.
// Candidates are bare digits, ordered by position and then by digit, at
// most [MaxSuggestions]. SuggestFix returns nil for valid lines, barcodes
// and malformed input.
func SuggestFix(line string) []string {
	d, err := clean(line)
	if err != nil {
		return nil
	}
	var ranges [][2]int
	switch len(d) {
	case BancarioLineLen:
		ranges = bancarioRanges
	case ArrecadacaoLineLen:
		ranges = arrecadacaoRanges
	default:
		return nil
	}
	if _, err := parseLine(d); err == nil {
		return nil
	}

	positions := searchPositions(d, ranges)
	var out []string
	buf := []byte(d)
	for _, i := range positions {
		orig := buf[i]
		for c := byte('0'); c <= '9'; c++ {
			if c == orig {
				continue
			}
			buf[i] = c
			if _, err := parseLine(string(buf)); err == nil {
				out = append(out, string(buf))
				if len(out) == MaxSuggestions {
					return out
				}
			}
		}
		buf[i] = orig
	}
	return out
}

// searchPositions returns the indexes of d where a single typo can explain
// the failures observed, so that SuggestFix only tries plausible fixes.
func searchPositions(d string, ranges [][2]int) []int {
	span := func(lo, hi int) []int {
		out := make([]int, 0, hi-lo)
		for i := lo; i < hi; i++ {
			out = append(out, i)
		}
		return out
	}
	failing, ok := failingFields(d)
	if len(d) == ArrecadacaoLineLen && (!ok || len(failing) > 1) {
		// A typo in the value-type digit changes the modulus of every
		// block, so it is the only single typo that breaks several blocks.
		return []int{2}
	}
	switch len(failing) {
	case 0:
		if len(d) == BancarioLineLen {
			// Modulo 10 catches every single-digit typo in fields 1–3,
			// so the typo is in field 4 (DAC) or field 5 (no own DV).
			return span(bancarioRanges[3][0], bancarioRanges[4][1])
		}
		return span(0, len(d))
	case 1:
		r := ranges[failing[0]-1]
		if len(d) == ArrecadacaoLineLen && r[0] > 2 {
			// A value-type typo may coincidentally break a single block.
			return append([]int{2}, span(r[0], r[1])...)
		}
		return span(r[0], r[1])
	default:
		return nil
	}
}

// failingFields returns the 1-based numbers of the fields whose own check
// digit does not match. ok is false for an arrecadação line with an invalid
// value-type identifier, for which no modulus is known.
func failingFields(d string) (failing []int, ok bool) {
	if len(d) == BancarioLineLen {
		for i, f := range bancarioFields {
			if mod10(d[f[0]:f[1]]) != d[f[1]] {
				failing = append(failing, i+1)
			}
		}
		return failing, true
	}
	dv, err := checkFunc(d[2])
	if err != nil {
		return nil, false
	}
	for i, r := range arrecadacaoRanges {
		if dv(d[r[0]:r[1]-1]) != d[r[1]-1] {
			failing = append(failing, i+1)
		}
	}
	return failing, true
}
