package ignitia

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
	"github.com/jw4/ignitia.go/pkg/collect"
)

// Option represents a function that can modify a session.
type Option func(*Session)

// BaseURL sets the base ignitia URL.
func BaseURL(u string) Option { return func(s *Session) { s.baseURL = u } }

// Credentials sets the ignitia login credentials.
func Credentials(username, password string) Option {
	return func(s *Session) { s.username, s.password = username, password }
}

// Assets configures the public assets root folder.
func Assets(path string) Option { return func(s *Session) { s.assets = path } }

// Templates configures the template files root folder.
func Templates(path string) Option { return func(s *Session) { s.templates = path } }

// NewSession returns a Session.
func NewSession(opts ...Option) *Session {
	ses := &Session{
		DebugWriter: os.Stdout,
		assets:      "public",
		templates:   "templates",
	}

	for _, opt := range opts {
		opt(ses)
	}

	ses.coll = collect.NewSession(ses.baseURL, ses.username, ses.password)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(ses.assets)))
	mux.HandleFunc("/index", ses.renderIndex)
	mux.HandleFunc("/report", ses.renderReport)
	ses.mux = mux

	return ses
}

// Session wraps a web session to ignitia.
type Session struct {
	DebugWriter io.Writer

	Error error

	Students []Student

	coll *collect.Session

	baseURL   string
	username  string
	password  string
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
		var courses []Course

		for _, course := range s.coll.Courses(student) {
			courses = append(courses, Course{course, s.coll.Assignments(student, course)})
		}

		s.Students = append(s.Students, Student{student, courses})
	}

	return s.coll.Error
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

var htmlHelpers = template.FuncMap{
	"htmlsafe": func(s string) template.HTML { return template.HTML(safehtml.HTMLEscaped(s).String()) },
	"rawhtml":  func(s string) template.HTML { return template.HTML(s) },
	"tolower":  strings.ToLower,
	"timenow":  func() string { return time.Now().Format(time.RFC3339) },
}
