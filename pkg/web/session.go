package web

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/safehtml"

	"github.com/jw4/ignitia.go/pkg/model"
)

// Option represents a function that can modify a session.
type Option func(*Session)

// Assets configures the public assets root folder.
func Assets(path string) Option { return func(s *Session) { s.assets = path } }

// Templates configures the template files root folder.
func Templates(path string) Option { return func(s *Session) { s.templates = path } }

// NewSession returns a Session.
func NewSession(collector Collector, opts ...Option) *Session {
	ses := &Session{
		DebugWriter: os.Stdout,
		assets:      "public",
		templates:   "templates",
		coll:        collector,
	}

	for _, opt := range opts {
		opt(ses)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(ses.assets)))
	mux.HandleFunc("/index", ses.renderIndex)
	mux.HandleFunc("/report", ses.renderReport)
	ses.mux = mux

	return ses
}

type Collector interface {
	Reset()
	Error() error
	Students() []model.Student
	Courses(model.Student) []model.Course
	Assignments(model.Student, model.Course) []model.Assignment
}

// Session wraps a web session to ignitia.
type Session struct {
	DebugWriter io.Writer
	Error       error
	Students    []model.Student

	coll Collector

	assets    string
	templates string

	mux http.Handler
}

func (s *Session) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.mux.ServeHTTP(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Refresh updates the cached data.
func (s *Session) Refresh() error {
	s.Error = nil
	s.Students = nil
	s.coll.Reset()

	for _, student := range s.coll.Students() {
		var courses []model.Course

		for _, course := range s.coll.Courses(student) {
			course.Assignments = s.coll.Assignments(student, course)
			courses = append(courses, course)
		}

		student.Courses = courses
		s.Students = append(s.Students, student)
	}

	return s.coll.Error()
}

// RenderHTML writes the report page out.
func (s *Session) RenderHTML(out io.Writer) error {
	pat := filepath.Join(s.templates, "*.gohtml")
	tmpl, err := template.New("report").Funcs(htmlHelpers).ParseGlob(pat)
	if err != nil {
		return fmt.Errorf("parsing %q: %v", pat, err)
	}

	if err := tmpl.Execute(out, s); err != nil {
		return err
	}

	return nil
}

func (s *Session) LastUpdate() string {
	var latest time.Time
	for _, student := range s.Students {
		for _, course := range student.Courses {
			for _, assignment := range course.Assignments {
				if assignment.AsOfTime().After(latest) {
					latest = assignment.AsOfTime()
				}
			}
		}
	}

	if latest.IsZero() {
		return "-n/a-"
	}

	return latest.In(time.Local).Format(time.RFC1123)
}

func (s *Session) renderIndex(writer http.ResponseWriter, _ *http.Request) {
	pat := filepath.Join(s.templates, "*.gohtml")
	tmpl, err := template.New("index").Funcs(htmlHelpers).ParseGlob(pat)
	if err != nil {
		s.renderError(writer, fmt.Errorf("parsing %q: %v", pat, err))
		return
	}

	if err := tmpl.Execute(writer, s); err != nil {
		s.renderError(writer, fmt.Errorf("executing template: %v", err))
		return
	}
}

func (s *Session) renderReport(writer http.ResponseWriter, _ *http.Request) {
	if err := s.Refresh(); err != nil {
		s.renderError(writer, err)
	}

	if err := s.RenderHTML(writer); err != nil {
		s.renderError(writer, err)
		return
	}
}

func (s *Session) renderError(writer http.ResponseWriter, err error) {
	writer.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(s.DebugWriter, "Error serving page: %v\n", err)
}

func tolower(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		case r >= 'a' && r <= 'z':
			return r
		case r == ' ':
			return '-'
		default:
			return -1
		}
	}, s)
}

var htmlHelpers = template.FuncMap{
	"htmlsafe": func(s string) template.HTML { return template.HTML(safehtml.HTMLEscaped(s).String()) },
	"rawhtml":  func(s string) template.HTML { return template.HTML(s) },
	"tolower":  tolower,
}
