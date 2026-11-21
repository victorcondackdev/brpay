package boleto

import (
	"errors"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestFactorBaseArithmetic(t *testing.T) {
	if got := factorBase.AddDate(0, 0, 1000); !got.Equal(date(2000, 7, 3)) {
		t.Errorf("base + 1000 = %v, want 2000-07-03", got)
	}
	if got := factorBase.AddDate(0, 0, 9999); !got.Equal(date(2025, 2, 21)) {
		t.Errorf("base + 9999 = %v, want 2025-02-21", got)
	}
	base2 := factorBase.AddDate(0, 0, FactorCycle)
	if !base2.Equal(date(2022, 5, 29)) {
		t.Errorf("second base = %v, want 2022-05-29", base2)
	}
	if got := base2.AddDate(0, 0, 1000); !got.Equal(date(2025, 2, 22)) {
		t.Errorf("base2 + 1000 = %v, want 2025-02-22", got)
	}
}

func TestDueDateAt(t *testing.T) {
	tests := []struct {
		name   string
		factor int
		ref    time.Time
		want   time.Time
	}{
		{"last day of cycle 1", 9999, date(2025, 2, 21), date(2025, 2, 21)},
		{"first day of cycle 2", 1000, date(2025, 2, 22), date(2025, 2, 22)},
		{"9999 seen after rollover", 9999, date(2025, 3, 15), date(2025, 2, 21)},
		{"1000 seen just before rollover", 1000, date(2025, 2, 10), date(2025, 2, 22)},
		{"1000 seen in 2005 is 2000", 1000, date(2005, 1, 1), date(2000, 7, 3)},
		{"1000 in the window gap picks nearest (2000)", 1000, date(2010, 1, 1), date(2000, 7, 3)},
		{"1000 in the window gap picks nearest (2025)", 1000, date(2012, 1, 1), date(2025, 2, 22)},
		{"cycle 2 mid-range", 1637, date(2026, 11, 21), date(2026, 11, 21)},
		{"old ref keeps cycle 1", 5000, date(2011, 1, 1), factorBase.AddDate(0, 0, 5000)},
		{"future ref uses cycle 3", 1000, date(2049, 10, 14), date(2049, 10, 14)},
		{"ref before base", 1500, date(1990, 1, 1), factorBase.AddDate(0, 0, 1500)},
		{"legacy factor", 500, date(2026, 1, 1), factorBase.AddDate(0, 0, 500)},
	}
	for _, tt := range tests {
		got, ok := DueDateAt(tt.factor, tt.ref)
		if !ok || !got.Equal(tt.want) {
			t.Errorf("%s: DueDateAt(%d, %s) = %s %v, want %s", tt.name, tt.factor, tt.ref.Format(time.DateOnly),
				got.Format(time.DateOnly), ok, tt.want.Format(time.DateOnly))
		}
	}
	for _, f := range []int{0, -1, 10000} {
		if _, ok := DueDateAt(f, date(2026, 1, 1)); ok {
			t.Errorf("DueDateAt(%d) should report no due date", f)
		}
	}
}

func TestDueDateUsesRefLocation(t *testing.T) {
	sp := time.FixedZone("BRT", -3*60*60)
	// 2025-02-22 01:00 UTC is still 2025-02-21 in São Paulo.
	ref := time.Date(2025, 2, 22, 1, 0, 0, 0, time.UTC).In(sp)
	f, err := FactorFor(ref)
	if err != nil || f != 9999 {
		t.Errorf("FactorFor(%v) = %d, %v; want 9999", ref, f, err)
	}
}

func TestFactorFor(t *testing.T) {
	tests := []struct {
		d    time.Time
		want int
	}{
		{date(2000, 7, 3), 1000},
		{date(2025, 2, 21), 9999},
		{date(2025, 2, 22), 1000},
		{date(2025, 2, 23), 1001},
		{date(2049, 10, 13), 9999},
		{date(2049, 10, 14), 1000},
	}
	for _, tt := range tests {
		if got, err := FactorFor(tt.d); err != nil || got != tt.want {
			t.Errorf("FactorFor(%s) = %d, %v; want %d", tt.d.Format(time.DateOnly), got, err, tt.want)
		}
	}
	for _, d := range []time.Time{{}, date(2000, 7, 2), date(1997, 10, 7)} {
		if _, err := FactorFor(d); !errors.Is(err, ErrDueDate) {
			t.Errorf("FactorFor(%v) = %v, want ErrDueDate", d, err)
		}
	}
}

// Every date from 2000 to 2060 must survive FactorFor → DueDateAt when the
// reference date is the due date itself or a few weeks around it.
func TestFactorRoundTrip(t *testing.T) {
	for d := date(2000, 7, 3); d.Before(date(2060, 1, 1)); d = d.AddDate(0, 0, 1) {
		f, err := FactorFor(d)
		if err != nil {
			t.Fatal(err)
		}
		for _, off := range []int{0, -30, 30} {
			ref := d.AddDate(0, 0, off)
			if got, ok := DueDateAt(f, ref); !ok || !got.Equal(d) {
				t.Fatalf("round trip %s (factor %d, ref %s) = %s", d.Format(time.DateOnly), f,
					ref.Format(time.DateOnly), got.Format(time.DateOnly))
			}
		}
	}
}
