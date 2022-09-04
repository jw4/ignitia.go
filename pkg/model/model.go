package model

import "log"

var handlers = map[string]func(string) (func(string) Full, bool){}

func RegisterHandler(name string, h func(string) (func(string) Full, bool)) {
	handlers[name] = h
}

func New(conn string) Full {
	for _, h := range handlers {
		if i, ok := h(conn); ok {
			return i(conn)
		}
	}

	log.Printf("no handler found for %q: %+v", conn, handlers)
	return nil
}

type Read interface {
	Students() []Student
	Courses(Student) []Course
	Assignments(Student, Course) []Assignment
}

type Write interface {
	Save(Read) error
	SaveStudents([]Student) error
	SaveCourses(Student, []Course) error
	SaveAssignments(Student, Course, []Assignment) error
}

type Full interface {
	Read
	Write
	Reset() error
	Error() error
}
