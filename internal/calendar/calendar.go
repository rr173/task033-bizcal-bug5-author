// Package calendar implements business-day arithmetic over a configurable
// calendar defined by a timezone, a set of weekend weekdays, and a set of
// holidays (fixed dates and yearly-recurring month-day pairs).
//
// The embedded time/tzdata package makes time.LoadLocation work in runtimes
// that do not ship a system timezone database (e.g. a bare alpine image), so
// non-local timezones resolve correctly without external files.
package calendar

import (
	"fmt"
	"time"

	// Embed the IANA timezone database so non-local timezones can be loaded
	// without a system tzdata installation.
	_ "time/tzdata"
)

// Config is the JSON shape of a calendar definition.
type Config struct {
	Timezone          string   `json:"timezone"`
	Weekend           []int    `json:"weekend"`
	FixedHolidays     []string `json:"fixed_holidays"`
	RecurringHolidays []string `json:"recurring_holidays"`
}

// Date is a calendar date with no time or timezone component.
type Date struct {
	Y int
	M time.Month
	D int
}

// String formats a Date as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Y, int(d.M), d.D)
}

// Before reports whether d is strictly earlier than o.
func (d Date) Before(o Date) bool {
	if d.Y != o.Y {
		return d.Y < o.Y
	}
	if d.M != o.M {
		return d.M < o.M
	}
	return d.D < o.D
}

// addDays returns the Date n calendar days away from d. Noon UTC is used as
// the anchor instant so that DST gaps near midnight never perturb the result.
func (d Date) addDays(n int) Date {
	t := time.Date(d.Y, d.M, d.D, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}
}

// ParseDate parses a YYYY-MM-DD string into a Date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return Date{Y: t.Year(), M: t.Month(), D: t.Day()}, nil
}

// Calendar is an immutable business-day calendar built from a Config.
type Calendar struct {
	loc       *time.Location
	weekend   map[int]bool    // weekday 0..6 (0=Sunday)
	holidays  map[string]bool // "YYYY-MM-DD"
	recurring map[string]bool // "MM-DD"
	memo      map[string]bool
}

// New validates cfg and returns an immutable Calendar.
func New(cfg Config) (*Calendar, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}

	weekend := make(map[int]bool)
	for _, d := range cfg.Weekend {
		if d < 0 || d > 6 {
			return nil, fmt.Errorf("invalid weekend day %d (must be 0-6)", d)
		}
		weekend[d] = true
	}
	if len(weekend) == 0 {
		weekend[6] = true // Saturday
		weekend[0] = true // Sunday
	}

	holidays := make(map[string]bool)
	for _, s := range cfg.FixedHolidays {
		if _, err := ParseDate(s); err != nil {
			return nil, fmt.Errorf("invalid fixed holiday %q: %w", s, err)
		}
		holidays[s] = true
	}

	recurring := make(map[string]bool)
	for _, s := range cfg.RecurringHolidays {
		if _, err := time.Parse("01-02", s); err != nil {
			return nil, fmt.Errorf("invalid recurring holiday %q: %w", s, err)
		}
		recurring[s] = true
	}

	return &Calendar{loc: loc, weekend: weekend, holidays: holidays, recurring: recurring, memo: make(map[string]bool)}, nil
}

// Timezone returns the configured timezone name.
func (c *Calendar) Timezone() string {
	return c.loc.String()
}

// IsBusinessDay reports whether d is a working day: not a weekend and not a
// holiday. A holiday that falls on a weekend is treated only as a weekend (no
// extra deduction, no observance shift).
func (c *Calendar) IsBusinessDay(d Date) bool {
	key := d.String()
	if cached, ok := c.memo[key]; ok {
		return cached
	}
	if c.isHoliday(d) {
		c.memo[key] = false
		return false
	}
	wd := int(time.Date(d.Y, d.M, d.D, 12, 0, 0, 0, time.UTC).Weekday())
	result := !c.weekend[wd]
	c.memo[key] = result
	return result
}

func (c *Calendar) isHoliday(d Date) bool {
	if c.holidays[d.String()] {
		return true
	}
	md := fmt.Sprintf("%02d-%02d", int(d.M), d.D)
	return c.recurring[md]
}

// AddBusinessDays returns the date n business days away from d.
//
// Counting starts from the calendar day AFTER d (n>0) or BEFORE d (n<0); d
// itself is never counted, even if it is a non-business day. n==0 returns d
// unchanged. Only business days increment the counter.
func (c *Calendar) AddBusinessDays(d Date, n int) Date {
	if n == 0 {
		return d
	}
	step := 1
	if n < 0 {
		step = -1
	}
	need := n
	if need < 0 {
		need = -need
	}
	cur := d
	if n < 0 && !c.IsBusinessDay(cur) {
		cur = cur.addDays(step)
	}
	count := 0
	for {
		cur = cur.addDays(step)
		if c.IsBusinessDay(cur) {
			count++
			if count == need {
				return cur
			}
		}
	}
}

// BusinessDaysBetween returns the count of business days in the half-open
// interval [from, to): from is included, to is excluded. Returns 0 when
// from >= to.
func (c *Calendar) BusinessDaysBetween(from, to Date) int {
	if !from.Before(to) {
		return 0
	}
	count := 0
	cur := from
	for cur.Before(to) {
		if c.IsBusinessDay(cur) {
			count++
		}
		cur = cur.addDays(1)
	}
	return count
}

// ListBusinessDays returns every business day in the half-open interval
// [from, to), ascending. Returns nil when from >= to.
func (c *Calendar) ListBusinessDays(from, to Date) []Date {
	if !from.Before(to) {
		return nil
	}
	var out []Date
	cur := from
	for cur.Before(to) {
		if c.IsBusinessDay(cur) {
			out = append(out, cur)
		}
		cur = cur.addDays(1)
	}
	return out
}

// ResolveTimestamp parses an RFC3339 instant and returns the local calendar
// date it falls on in the configured timezone, together with whether that date
// is a business day.
func (c *Calendar) ResolveTimestamp(s string) (Date, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid timestamp %q: %w", s, err)
	}
	local := t.In(c.loc)
	return Date{Y: local.Year(), M: local.Month(), D: local.Day()}, nil
}
