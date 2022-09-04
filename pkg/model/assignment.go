package model

import (
	"fmt"
	"time"
)

type Assignment struct {
	ID        int    `json:"id"`
	Unit      int    `json:"unit"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Progress  int    `json:"progress"`
	Due       string `json:"due"`
	Completed string `json:"completed"`
	Score     int    `json:"score"`
	Status    string `json:"status"`
	AsOf      string `json:"as_of"`
}

func (a *Assignment) String() string {
	return fmt.Sprintf("Unit: %d, %s, %q, Due: %s, Status: %s", a.Unit, a.Type, a.Title, a.Due, a.Status)
}

func (a *Assignment) AsOfTime() time.Time     { return parseTime(a.AsOf) }
func (a *Assignment) CompleteDate() time.Time { return parseDate(a.Completed) }
func (a *Assignment) DueDate() time.Time      { return parseDate(a.Due) }

func (a *Assignment) IsIncomplete() bool {
	switch a.Status {
	case "Skipped", "Completed", "Graded":
		return false
	default:
		return true
	}
}

func (a *Assignment) IsCurrent() bool {
	if a.DueDate().After(thisWeek()) && a.DueDate().Before(nextWeek()) {
		return true
	}

	if a.CompleteDate().After(thisWeek()) && a.CompleteDate().Before(nextWeek()) {
		return true
	}

	return false
}

func (a *Assignment) IsFuture() bool { return a.DueDate().After(tomorrow()) }
func (a *Assignment) IsPast() bool   { return a.DueDate().Before(today()) }
func (a *Assignment) IsDue() bool {
	if !a.IsIncomplete() {
		return false
	}

	const finished = 100
	if a.Progress == finished {
		return false
	}

	if a.DueDate().Before(tomorrow()) {
		return true
	}

	return false
}

func (a *Assignment) IsOverdue() bool {
	if !a.IsDue() {
		return false
	}

	if a.DueDate().Before(ago(7)) {
		return true
	}

	return false
}

func today() time.Time {
	y, m, d := time.Now().Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func tomorrow() time.Time { return in(1) }

func ago(days int) time.Time { return in(-days) }

func in(days int) time.Time {
	const day = 24

	return today().Add(time.Duration(days) * day * time.Hour)
}

func thisWeek() time.Time {
	cur := today()

	offset := int(time.Monday - cur.Weekday())
	if offset > 0 {
		const weekBegin = -6
		offset = weekBegin
	}

	return cur.AddDate(0, 0, offset)
}

func nextWeek() time.Time {
	const daysInWeek = 7

	return thisWeek().AddDate(0, 0, daysInWeek)
}

func parseDate(s string) time.Time {
	for _, fmt := range []string{"2006-01-02", "01/02/2006"} {
		dt, err := time.ParseInLocation(fmt, s, time.Local)
		if err == nil {
			return dt
		}
	}

	return time.Time{}
}

func parseTime(s string) time.Time {
	for _, fmt := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC822, "2006-01-02", "01/02/2006"} {
		dt, err := time.ParseInLocation(fmt, s, time.UTC)
		if err == nil {
			return dt
		}
	}

	return time.Time{}
}
