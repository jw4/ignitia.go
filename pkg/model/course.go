package model

type Course struct {
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	Assignments []Assignment `json:"-"`
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

func (c *Course) DueAssignments() int {
	var due int

	for _, a := range c.Assignments {
		if a.IsDue() {
			due++
		}
	}

	return due
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
