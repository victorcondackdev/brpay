package pix

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseEndToEndID(t *testing.T) {
	const s = "E00000000202611211430abcdeFGHIJ1"
	id, err := ParseEndToEndID(s)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 21, 14, 30, 0, 0, time.UTC)
	if id.ISPB != "00000000" || !id.Time.Equal(want) || id.Sequence != "abcdeFGHIJ1" || id.String() != s {
		t.Errorf("ParseEndToEndID = %+v (%s)", id, id)
	}

	invalid := []struct{ in, msg string }{
		{"", "length 0"},
		{"X00000000202611211430abcdeFGHIJ1", `start with "E"`},
		{"E0000000A202611211430abcdeFGHIJ1", "ISPB"},
		{"E000000002026112114X0abcdeFGHIJ1", "12 digits"},
		{"E00000000202613211430abcdeFGHIJ1", "not a valid date"},
		{"E00000000202602301430abcdeFGHIJ1", "not a valid date"},
		{"E00000000202611211430abcde-GHIJ1", "alphanumeric"},
	}
	for _, tt := range invalid {
		if _, err := ParseEndToEndID(tt.in); !errors.Is(err, ErrInvalidEndToEndID) || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("ParseEndToEndID(%q) = %v, want error containing %q", tt.in, err, tt.msg)
		}
	}
}

func TestNewEndToEndID(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	at := time.Date(2026, 11, 21, 11, 30, 59, 0, loc) // 14:30 UTC

	id, err := NewEndToEndID("60701190", at, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseEndToEndID(id)
	if err != nil {
		t.Fatalf("ParseEndToEndID(%q): %v", id, err)
	}
	if p.ISPB != "60701190" || !p.Time.Equal(time.Date(2026, 11, 21, 14, 30, 0, 0, time.UTC)) {
		t.Errorf("round trip = %+v", p)
	}
	if id2, _ := NewEndToEndID("60701190", at, nil); id2 == id {
		t.Error("two ids with crypto/rand collided")
	}

	// Deterministic reader: bytes >= 248 are skipped, the rest map mod 62.
	r := bytes.NewReader(append([]byte{255, 248, 0, 1, 61, 62, 123}, make([]byte, 64)...))
	id, err = NewEndToEndID("00000000", at, r)
	if err != nil || id[21:] != "01z0z000000" {
		t.Errorf("deterministic sequence = %q, %v", id, err)
	}

	tests := []struct {
		name string
		ispb string
		t    time.Time
		r    io.Reader
		msg  string
	}{
		{"short ispb", "123", at, nil, "ISPB"},
		{"letters ispb", "1234567A", at, nil, "ISPB"},
		{"year", "00000000", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), nil, "year"},
		{"negative year", "00000000", time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC), nil, "year"},
		{"reader error", "00000000", at, strings.NewReader("short"), "random bytes"},
		{"unusable reader", "00000000", at, bytes.NewReader(bytes.Repeat([]byte{0xFF}, 2000)), "no usable bytes"},
	}
	for _, tt := range tests {
		if id, err := NewEndToEndID(tt.ispb, tt.t, tt.r); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%s: NewEndToEndID = %q, %v; want error containing %q", tt.name, id, err, tt.msg)
		}
	}
}
