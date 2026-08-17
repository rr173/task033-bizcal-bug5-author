package calendar

import "testing"

func testCal(t *testing.T) *Calendar {
	t.Helper()
	c, err := New(Config{
		Timezone:          "Asia/Shanghai",
		Weekend:           []int{6, 0},
		FixedHolidays:     []string{"2026-01-01"},
		RecurringHolidays: []string{"01-01", "05-01", "10-01"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func mustDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"2026-08-15", "2026-08-15", false},
		{"2026-01-01", "2026-01-01", false},
		{"2026-8-5", "", true},   // not zero-padded
		{"2026/08/15", "", true}, // wrong separator
		{"not-a-date", "", true},
		{"2026-13-01", "", true}, // bad month
	}
	for _, tc := range cases {
		d, err := ParseDate(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseDate(%q): want error, got %s", tc.in, d)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDate(%q): unexpected error %v", tc.in, err)
			continue
		}
		if d.String() != tc.want {
			t.Errorf("ParseDate(%q): got %s want %s", tc.in, d, tc.want)
		}
	}
}

func TestIsBusinessDay(t *testing.T) {
	c := testCal(t)
	cases := []struct {
		date string
		want bool
	}{
		{"2026-08-14", true},  // Friday
		{"2026-08-15", false}, // Saturday
		{"2026-08-16", false}, // Sunday
		{"2026-08-17", true},  // Monday
		{"2026-01-01", false}, // Thursday but holiday (fixed + recurring)
		{"2027-01-01", false}, // recurring holiday next year
		{"2026-05-01", false}, // recurring holiday (Labor Day, Friday)
		{"2026-10-01", false}, // recurring holiday (National Day, Thursday)
		{"2026-10-02", true},  // Friday, not a holiday
	}
	for _, tc := range cases {
		d := mustDate(tc.date)
		if got := c.IsBusinessDay(d); got != tc.want {
			t.Errorf("IsBusinessDay(%s) = %v want %v", tc.date, got, tc.want)
		}
	}
}

func TestAddBusinessDays(t *testing.T) {
	c := testCal(t)
	cases := []struct {
		date string
		n    int
		want string
	}{
		{"2026-08-14", 1, "2026-08-17"},  // Fri +1 -> Mon
		{"2026-08-14", 0, "2026-08-14"},  // n=0 unchanged (Friday)
		{"2026-08-15", 0, "2026-08-15"},  // n=0 unchanged (weekend)
		{"2026-08-15", 1, "2026-08-17"},  // Sat +1 -> Mon (skips Sun)
		{"2026-08-17", -1, "2026-08-14"}, // Mon -1 -> Fri
		{"2026-08-14", 5, "2026-08-21"},  // Fri +5 -> Fri (17,18,19,20,21)
		{"2026-01-01", 1, "2026-01-02"},  // holiday Thu +1 -> Fri (non-holiday)
		{"2026-04-30", 1, "2026-05-04"},  // Thu +1 skips May 1 (holiday), 2/3 (weekend) -> Mon
	}
	for _, tc := range cases {
		got := c.AddBusinessDays(mustDate(tc.date), tc.n).String()
		if got != tc.want {
			t.Errorf("AddBusinessDays(%s, %d) = %s want %s", tc.date, tc.n, got, tc.want)
		}
	}
}

func TestBusinessDaysBetween(t *testing.T) {
	c := testCal(t)
	cases := []struct {
		from, to string
		want     int
	}{
		{"2026-08-10", "2026-08-20", 8},  // [Mon, Thu): 8 business days
		{"2026-08-10", "2026-08-10", 0},  // equal
		{"2026-08-20", "2026-08-10", 0},  // reversed
		{"2026-08-14", "2026-08-17", 1},  // [Fri, Mon): only Fri
		{"2026-04-30", "2026-05-05", 2},  // Apr 30 + May 4 (May 1 holiday, 2/3 weekend, 5 excluded)
		{"2026-08-15", "2026-08-17", 0},  // [Sat, Mon): Sat/Sun only, both excluded -> 0
	}
	for _, tc := range cases {
		got := c.BusinessDaysBetween(mustDate(tc.from), mustDate(tc.to))
		if got != tc.want {
			t.Errorf("BusinessDaysBetween(%s, %s) = %d want %d", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestListBusinessDays(t *testing.T) {
	c := testCal(t)
	want := []string{"2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14"}
	days := c.ListBusinessDays(mustDate("2026-08-10"), mustDate("2026-08-17"))
	if len(days) != len(want) {
		t.Fatalf("ListBusinessDays: got %d dates want %d (%v)", len(days), len(want), days)
	}
	for i, d := range days {
		if d.String() != want[i] {
			t.Errorf("ListBusinessDays[%d] = %s want %s", i, d, want[i])
		}
	}
	// Reversed range returns nil.
	if got := c.ListBusinessDays(mustDate("2026-08-17"), mustDate("2026-08-10")); got != nil {
		t.Errorf("ListBusinessDays reversed: want nil, got %v", got)
	}
	// Equal range returns nil.
	if got := c.ListBusinessDays(mustDate("2026-08-10"), mustDate("2026-08-10")); got != nil {
		t.Errorf("ListBusinessDays equal: want nil, got %v", got)
	}
}

func TestResolveTimestamp(t *testing.T) {
	c := testCal(t)
	cases := []struct {
		ts      string
		wantDay string
		wantBiz bool
	}{
		{"2026-08-15T16:30:00Z", "2026-08-16", false},      // +8 -> next day, Sunday
		{"2026-08-15T07:00:00+08:00", "2026-08-15", false}, // offset form, Saturday
		{"2026-08-14T00:00:00Z", "2026-08-14", true},       // +8 -> 08:00 Friday
	}
	for _, tc := range cases {
		d, err := c.ResolveTimestamp(tc.ts)
		if err != nil {
			t.Errorf("ResolveTimestamp(%s): unexpected error %v", tc.ts, err)
			continue
		}
		if d.String() != tc.wantDay {
			t.Errorf("ResolveTimestamp(%s): date = %s want %s", tc.ts, d, tc.wantDay)
		}
		if c.IsBusinessDay(d) != tc.wantBiz {
			t.Errorf("ResolveTimestamp(%s): business = %v want %v", tc.ts, c.IsBusinessDay(d), tc.wantBiz)
		}
	}
}

func TestResolveTimestampNewYork(t *testing.T) {
	// Validates that a non-local timezone loads (embedded tzdata). August is
	// EDT (UTC-4).
	ny, err := New(Config{Timezone: "America/New_York", Weekend: []int{6, 0}})
	if err != nil {
		t.Fatalf("New America/New_York: %v", err)
	}
	d, err := ny.ResolveTimestamp("2026-08-15T04:00:00Z")
	if err != nil {
		t.Fatalf("resolve 04:00Z: %v", err)
	}
	if d.String() != "2026-08-15" {
		t.Errorf("resolve 04:00Z: got %s want 2026-08-15", d)
	}
	d2, err := ny.ResolveTimestamp("2026-08-15T03:00:00Z")
	if err != nil {
		t.Fatalf("resolve 03:00Z: %v", err)
	}
	if d2.String() != "2026-08-14" {
		t.Errorf("resolve 03:00Z: got %s want 2026-08-14", d2)
	}
}

func TestNewErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"bad timezone", Config{Timezone: "Bogus/Zone"}},
		{"bad weekend day", Config{Timezone: "Asia/Shanghai", Weekend: []int{7}}},
		{"bad fixed holiday", Config{Timezone: "Asia/Shanghai", FixedHolidays: []string{"not-a-date"}}},
		{"bad recurring holiday", Config{Timezone: "Asia/Shanghai", RecurringHolidays: []string{"13-01"}}},
	}
	for _, tc := range cases {
		if _, err := New(tc.cfg); err == nil {
			t.Errorf("New(%s): want error, got nil", tc.name)
		}
	}
}

func TestNewDefaultWeekend(t *testing.T) {
	// An empty weekend falls back to Saturday+Sunday.
	c, err := New(Config{Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !c.IsBusinessDay(mustDate("2026-08-14")) { // Friday
		t.Error("default weekend: Friday should be a business day")
	}
	if c.IsBusinessDay(mustDate("2026-08-15")) { // Saturday
		t.Error("default weekend: Saturday should not be a business day")
	}
	if c.IsBusinessDay(mustDate("2026-08-16")) { // Sunday
		t.Error("default weekend: Sunday should not be a business day")
	}
}

func TestDateBefore(t *testing.T) {
	if !mustDate("2026-08-10").Before(mustDate("2026-08-11")) {
		t.Error("08-10 before 08-11: want true")
	}
	if mustDate("2026-08-11").Before(mustDate("2026-08-11")) {
		t.Error("equal Before: want false")
	}
	if mustDate("2026-08-12").Before(mustDate("2026-08-11")) {
		t.Error("later before earlier: want false")
	}
	if !mustDate("2025-12-31").Before(mustDate("2026-01-01")) {
		t.Error("cross-year Before: want true")
	}
}
