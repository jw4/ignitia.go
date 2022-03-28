package ignitia

import "github.com/jw4/ignitia.go/pkg/collect"

type Student struct {
	collect.Student
	Courses []Course `json:"-"`
}

func (s *Student) IncompleteCourses() int {
	var incomplete int
	for _, c := range s.Courses {
		if c.IncompleteAssignments() > 0 {
			incomplete++
		}
	}
	return incomplete
}

func (s *Student) DueAssignments() int {
	var due int
	for _, c := range s.Courses {
		due += c.DueAssignments()
	}
	return due
}

func (s *Student) OverdueAssignments() int {
	var overdue int
	for _, c := range s.Courses {
		overdue += c.OverdueAssignments()
	}
	return overdue
}
