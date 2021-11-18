package ignitia

type Student struct {
	ID          int       `json:"id"`
	DisplayName string    `json:"displayName"`
	Courses     []*Course `json:"-"`
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

func (s *Student) OverdueAssignments() int {
	var overdue int
	for _, c := range s.Courses {
		overdue += c.OverdueAssignments()
	}
	return overdue
}
