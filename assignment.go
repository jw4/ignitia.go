package ignitia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

func ToAssignment(raw map[string]interface{}) (*Assignment, error) {
	id, ok := raw["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type for cell id: got %T (%v), expected number", raw["id"], raw["id"])
	}
	assignment := &Assignment{ID: int(id)}

	items, ok := raw["cell"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected type for cell values: got %T (%v), expected []interface{}", raw["cell"], raw["cell"])
	}

	if len(items) != 9 {
		return nil, fmt.Errorf("unexpected length of items, got %d (%v), expected 9", len(items), items)
	}

	_, ok = items[0].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type for ID (items[0]): got %T (%v), expected number", items[0], items[0])
	}

	unit, ok := items[1].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Unit (items[1]): got %T (%v), expected number", items[1], items[1])
	}
	assignment.Unit = int(unit)

	title, ok := items[2].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Title (items[2]): got %T (%v), expected string", items[2], items[2])
	}
	assignment.Title = title

	typ, ok := items[3].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Type (items[3]): got %T (%v), expected string", items[3], items[3])
	}
	assignment.Type = typ

	progress, ok := items[4].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Progress (items[4]): got %T (%v), expected number", items[4], items[4])
	}
	assignment.Progress = int(progress)

	switch due := items[5].(type) {
	case string:
		assignment.Due = due
	case nil:
	default:
		return nil, fmt.Errorf("unexpected type for Due (items[5]): got %T (%v), expected string", items[5], items[5])
	}

	switch completed := items[6].(type) {
	case string:
		assignment.Completed = completed
	case nil:
	default:
		return nil, fmt.Errorf("unexpected type for Completed (items[6]): got %T (%v), expected string", items[6], items[6])
	}

	score, ok := items[7].(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Score (items[7]): got %T (%v), expected number", items[7], items[7])
	}
	assignment.Score = int(score)

	status, ok := items[8].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type for Status (items[8]): got %T (%v), expected string", items[8], items[8])
	}
	assignment.Status = status

	return assignment, nil
}

type Assignment struct {
	ID        int
	Unit      int
	Title     string
	Type      string
	Progress  int
	Due       string
	Completed string
	Score     int
	Status    string
}

func (a *Assignment) String() string {
	return fmt.Sprintf("Unit: %d, %s, %q, Due: %s, Status: %s", a.Unit, a.Type, a.Title, a.Due, a.Status)
}

func (a *Assignment) CompleteDate() time.Time { return parseDate(a.Completed) }

func (a *Assignment) DueDate() time.Time { return parseDate(a.Due) }

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

	if a.Progress == 100 {
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

	if a.DueDate().Before(today()) {
		return true
	}

	return false
}

func today() time.Time {
	y, m, d := time.Now().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func tomorrow() time.Time {
	return today().Add(24 * time.Hour)
}

func yesterday() time.Time {
	return today().Add(-24 * time.Hour)
}

func thisWeek() time.Time {
	cur := today()
	offset := int(time.Monday - cur.Weekday())
	if offset > 0 {
		offset = -6
	}
	return cur.AddDate(0, 0, offset)
}

func nextWeek() time.Time {
	return thisWeek().AddDate(0, 0, 7)
}

func parseDate(s string) time.Time {
	dt, err := time.ParseInLocation("01/02/2006", s, time.Local)
	if err != nil {
		return time.Time{}
	}
	return dt
}

type assignmentResponseHelper struct {
	Page        int
	Total       int
	Records     int
	Assignments []*Assignment
}

func (a *assignmentResponseHelper) UnmarshalJSON(b []byte) error {
	type responseType struct {
		Page        interface{} `json:"page"`
		Total       int         `json:"total"`
		Records     int         `json:"records"`
		Assignments interface{} `json:"rows"`
	}
	r := responseType{}
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&r); err != nil {
		return err
	}

	a.Total = r.Total
	a.Records = r.Records

	switch v := r.Page.(type) {
	case float64:
		a.Page = int(v)
	case int:
		a.Page = v
	case string:
		p, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("unexpected value for Page: expected number got %v: %v", v, err)
		}
		a.Page = p
	default:
		return fmt.Errorf("unexpected type for Page: got %T (%v), expected int or string", v, v)
	}

	switch rows := r.Assignments.(type) {
	case map[string]interface{}:
		if len(rows) > 0 {
			return fmt.Errorf("unexpected type for rows: got map with %d keys, expected []interface{} or empty map", len(rows))
		}
	case []interface{}:
		assignments := map[int]*Assignment{}
		for _, v := range rows {
			switch cell := v.(type) {
			case map[string]interface{}:
				assignment, err := ToAssignment(cell)
				if err != nil {
					return err
				}

				assignments[assignment.ID] = assignment
			default:
				return fmt.Errorf("unexpected type for cells: got %T, expected map[string]interface{}", cell)
			}
		}

		list := make([]*Assignment, 0, len(assignments))
		for _, v := range assignments {
			list = append(list, v)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].DueDate().Before(list[j].DueDate()) })

		a.Assignments = list
	default:
		return fmt.Errorf("unexpected type for rows: got %T, expected map[string]interface{} or []interface{}", rows)
	}

	return nil
}
