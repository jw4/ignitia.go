package ignitia

import "github.com/jw4/ignitia.go/pkg/collect"

type Course struct {
	collect.Course
	Assignments []collect.Assignment `json:"-"`
}

func (c *Course) IncompleteAssignments() int {
	var incomplete int

	for _, a := range c.Assignments {
		if a.IsIncomplete() {
			incomplete++
		}
	}

	return incomplete
}

func (c *Course) OverdueAssignments() int {
	var overdue int

	for _, a := range c.Assignments {
		if a.IsOverdue() {
			overdue++
		}
	}

	return overdue
}
