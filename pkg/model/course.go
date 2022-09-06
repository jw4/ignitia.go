package model

import "sort"

type Course struct {
	ID          int                 `json:"id"`
	StudentID   int                 `json:"student_id"`
	Title       string              `json:"title"`
	Assignments map[int]*Assignment `json:"assignments"`
}

func (c *Course) SortedAssignments() []*Assignment {
	var assignments []*Assignment
	for _, assignment := range c.Assignments {
		assignments = append(assignments, assignment)
	}

	sort.Slice(assignments, func(x, y int) bool {
		lhs, rhs := assignments[x], assignments[y]
		if lhs.Unit == rhs.Unit {
			if lhs.Due == rhs.Due {
				if lhs.Completed == rhs.Completed {
					return lhs.Title < rhs.Title
				}

				return lhs.CompleteDate().Before(rhs.CompleteDate())
			}

			return lhs.DueDate().Before(rhs.DueDate())
		}

		return lhs.Unit < rhs.Unit
	})

	return assignments
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
