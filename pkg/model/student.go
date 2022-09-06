package model

import "sort"

type Student struct {
	ID          int             `json:"id"`
	DisplayName string          `json:"displayName"`
	Courses     map[int]*Course `json:"courses"`
}

func (s *Student) SortedCourses() []*Course {
	var courses []*Course
	for _, course := range s.Courses {
		courses = append(courses, course)
	}

	sort.Slice(courses, func(x, y int) bool { return courses[x].Title < courses[y].Title })

	return courses
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
