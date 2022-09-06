package model

import (
	"log"
	"sort"
	"time"
)

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
	Data() Data
}

type Write interface {
	Save(Read) error
}

type Full interface {
	Read
	Write
	Reset() error
	Error() error
}

type Data struct {
	Students map[int]*Student `json:"students"`
	Errors   []error          `json:"errors"`
	AsOf     time.Time        `json:"as_of"`
}

func (d *Data) SortedStudents() []*Student {
	var students []*Student
	for _, student := range d.Students {
		students = append(students, student)
	}

	sort.Slice(students, func(x, y int) bool { return students[x].DisplayName < students[y].DisplayName })

	return students
}

func (d *Data) LastUpdate() string {
	if d.AsOf.IsZero() {
		return "- n/a -"
	}

	return d.AsOf.Format(time.RFC1123)
}
