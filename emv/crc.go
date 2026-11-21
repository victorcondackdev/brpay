package emv

// CRCPrefix is the ID and length of the CRC field ("63", "04") that precede
// the checksum at the end of every payload. The checksum covers it.
const CRCPrefix = "6304"

// crcTable is the byte-wise lookup table for CRC-16/CCITT-FALSE.
var crcTable = makeCRCTable()

func makeCRCTable() (t [256]uint16) {
	for i := range t {
		c := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if c&0x8000 != 0 {
				c = c<<1 ^ 0x1021
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return t
}

// Checksum returns the CRC-16/CCITT-FALSE of data (polynomial 0x1021,
// initial value 0xFFFF, no reflection, no final XOR). It does not allocate.
func Checksum(data string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(data); i++ {
		crc = crc<<8 ^ crcTable[byte(crc>>8)^data[i]]
	}
	return crc
}

const hexDigits = "0123456789ABCDEF"

// CRC16 returns the checksum of data as 4 uppercase hexadecimal digits,
// the representation used in field 63.
func CRC16(data string) string {
	c := Checksum(data)
	b := [4]byte{hexDigits[c>>12], hexDigits[c>>8&0xF], hexDigits[c>>4&0xF], hexDigits[c&0xF]}
	return string(b[:])
}

// AppendCRC appends the CRC field to a payload that does not have one yet:
// it adds "6304" and the checksum computed over everything including "6304".
// The input is not inspected; pass it without any field 63.
func AppendCRC(payload string) string {
	p := payload + CRCPrefix
	return p + CRC16(p)
}

// VerifyCRC checks that payload ends with a CRC field ("6304" plus 4 hex
// digits) whose value matches the checksum of everything before those 4
// digits. Hex digits are compared case-insensitively. It returns a
// *ParseError if the field is missing and a *CRCError on mismatch.
// It does not allocate on success.
func VerifyCRC(payload string) error {
	n := len(payload)
	if n < 8 || payload[n-8:n-4] != CRCPrefix {
		off := n - 8
		if off < 0 {
			off = 0
		}
		return &ParseError{Offset: off, ID: "63", Reason: "payload does not end with a CRC field (6304XXXX)", Err: ErrStructure}
	}
	got := payload[n-4:]
	want := Checksum(payload[:n-4])
	if v, ok := parseHex16(got); !ok || v != want {
		return &CRCError{Got: got, Want: CRC16(payload[:n-4])}
	}
	return nil
}

func parseHex16(s string) (uint16, bool) {
	var v uint16
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			c -= '0'
		case c >= 'A' && c <= 'F':
			c -= 'A' - 10
		case c >= 'a' && c <= 'f':
			c -= 'a' - 10
		default:
			return 0, false
		}
		v = v<<4 | uint16(c)
	}
	return v, true
}
