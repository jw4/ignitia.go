package ignitia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/google/safehtml"
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

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(ses.assets)))
	mux.HandleFunc("/index", ses.renderIndex)
	mux.HandleFunc("/report", ses.renderReport)
	ses.mux = mux

	return ses
}

// Session wraps a web session to ignitia.
type Session struct {
	Students []*Student

	DebugWriter io.Writer

	Error     error
	baseURL   string
	username  string
	password  string
	assets    string
	templates string

	collector *colly.Collector

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
	s.Error = s.init()

	s.getAndUpdate(fmt.Sprintf("%s/owsoo/parent/populateStudents?_=%d", s.baseURL, ts()), s.loadStudentsFromJSON)

	for _, student := range s.Students {
		s.getAndUpdate(
			fmt.Sprintf("%s/owsoo/parent/populateCourses?student_id=%d&_=%d", s.baseURL, student.ID, ts()),
			s.loadCoursesFromJSON(student))
	}

	for _, student := range s.Students {
		for _, course := range student.Courses {
			data := map[string]string{
				"student_id":    fmt.Sprintf("%d", student.ID),
				"enrollment_id": fmt.Sprintf("%d", course.ID),
				"nd":            fmt.Sprintf("%d", ts()),
				"rows":          "1000",
				"page":          "1",
			}
			s.postAndUpdate(
				fmt.Sprintf("%s/owsoo/parent/listAssignmentsByCourse", s.baseURL),
				data, s.loadAssignmentsFromJSON(course))
		}
	}

	return s.Error
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

func (s *Session) init() error {
	s.collector = colly.NewCollector(
		colly.UserAgent("report-generator v0.0.1 (johnweldon4@gmail.com)"),
		colly.TraceHTTP(),
	)

	s.collector.SetRequestTimeout(30 * time.Second)

	s.collector.OnRequest(s.logRequest)
	s.collector.OnResponse(s.logResponse)

	s.collector.OnHTML("#loginForm", s.login)
	s.collector.OnHTML(".login-error", s.loginError)

	if err := s.collector.Visit(s.baseURL); err != nil {
		return err
	}

	return s.Error
}

func (s *Session) login(element *colly.HTMLElement) {
	data := map[string]string{
		"j_username": s.username,
		"j_password": s.password,
	}

	if err := element.Request.Post(element.Attr("action"), data); err != nil {
		s.Error = err
	}
}

func (s *Session) loginError(element *colly.HTMLElement) {
	s.Error = fmt.Errorf("error logging in: %s", element.Text)
}

func (s *Session) loadStudentsFromJSON(response *colly.Response) {
	if s.Error != nil {
		return
	}

	if !(assertOK(response) && assertJSON(response)) {
		return
	}

	var students []*Student
	if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&students); err != nil {
		s.Error = err
		return
	}

	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	s.Students = students
}

func (s *Session) loadCoursesFromJSON(student *Student) func(*colly.Response) {
	return func(response *colly.Response) {
		if s.Error != nil {
			return
		}

		if !(assertOK(response) && assertJSON(response)) {
			return
		}

		var courses []*Course
		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&courses); err != nil {
			s.Error = err
			return
		}

		sort.Slice(courses, func(i, j int) bool { return courses[i].ID < courses[j].ID })
		student.Courses = courses
	}
}

func (s *Session) loadAssignmentsFromJSON(course *Course) func(*colly.Response) {
	return func(response *colly.Response) {
		if s.Error != nil {
			return
		}

		if !(assertOK(response) && assertJSON(response)) {
			return
		}

		var helper assignmentResponseHelper
		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&helper); err != nil {
			s.Error = err
			return
		}

		course.Assignments = helper.Assignments
	}
}

func (s *Session) getAndUpdate(link string, loader func(*colly.Response)) {
	if s.Error != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Visit(link); err != nil {
		s.Error = err
	}
}

func (s *Session) postAndUpdate(link string, data map[string]string, loader func(*colly.Response)) {
	if s.Error != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Post(link, data); err != nil {
		s.Error = err
	}
}

func (s *Session) logRequest(request *colly.Request) {
	fmt.Fprintf(s.DebugWriter, "%d %s %s\n", request.ID, request.Method, request.URL)
}

func (s *Session) logResponse(response *colly.Response) {
	fmt.Fprintf(s.DebugWriter, "%d %d %s %s\n",
		response.Request.ID,
		response.StatusCode,
		response.Trace.FirstByteDuration,
		response.Trace.ConnectDuration)
	if response.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(s.DebugWriter, "%d %d\n---\n%s\n---\n",
			response.Request.ID,
			response.StatusCode,
			string(response.Body))
	}
}

func assertOK(r *colly.Response) bool {
	if r.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(os.Stderr, "Unexpected error: %d (%v)\n---\n%s\n---\n", r.StatusCode, r.Headers, string(r.Body))
		return false
	}
	return true
}

func assertJSON(r *colly.Response) bool {
	mt, _, err := mime.ParseMediaType(r.Headers.Get("content-type"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing Media Type %q: %v\n", r.Headers.Get("content-type"), err)
		return false
	}

	if mt != "application/json" {
		fmt.Fprintf(os.Stderr, "Media Type %q\n---\n%s\n---\n", mt, string(r.Body))
		return false
	}

	return true
}

func ts() int64 { return time.Now().Unix() }

var htmlHelpers = template.FuncMap{
	"htmlsafe": func(s string) template.HTML { return template.HTML(safehtml.HTMLEscaped(s).String()) }, // nolint: gosec
	"tolower":  strings.ToLower,
	"timenow":  func() string { return time.Now().Format(time.RFC3339) },
}
