package ignitia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

func NewSession(baseURL, username, password, assets string) *Session {
	ses := &Session{
		DebugWriter: os.Stdout,

		baseURL:  baseURL,
		username: username,
		password: password,

		reportTmpl: template.Must(template.New("report").Funcs(htmlHelpers).Parse(reportTemplate)),
		headerTmpl: template.Must(template.New("header").Funcs(htmlHelpers).Parse(headerTemplate)),
		footerTmpl: template.Must(template.New("footer").Funcs(htmlHelpers).Parse(footerTemplate)),
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir(assets))))
	mux.HandleFunc("/", ses.renderReport)
	ses.mux = mux

	return ses
}

type Session struct {
	Students []*Student

	DebugWriter io.Writer

	err      error
	baseURL  string
	username string
	password string

	collector     *colly.Collector
	lastRefreshed time.Time

	mux http.Handler

	reportTmpl *template.Template
	headerTmpl *template.Template
	footerTmpl *template.Template
}

func (s *Session) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.mux.ServeHTTP(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Session) Refresh() error {
	if s.collector == nil {
		s.err = s.init()
	}

	if len(s.Students) == 0 || s.lastRefreshed.Before(time.Now().Add(-10*time.Minute)) {
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

		s.lastRefreshed = time.Now()
	}

	return s.err
}

func (s *Session) RenderHTML(out io.Writer) error { return s.reportTmpl.Execute(out, s) }

func (s *Session) renderReport(writer http.ResponseWriter, _ *http.Request) {
	if err := s.Refresh(); err != nil {
		s.renderError(writer, err)
		return
	}

	if err := s.headerTmpl.Execute(writer, s); err != nil {
		s.renderError(writer, err)
		return
	}
	if err := s.reportTmpl.Execute(writer, s); err != nil {
		s.renderError(writer, err)
		return
	}
	if err := s.footerTmpl.Execute(writer, s); err != nil {
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

	s.collector.OnRequest(s.logRequest)
	s.collector.OnResponse(s.logResponse)

	s.collector.OnHTML("#loginForm", s.login)

	return s.collector.Visit(s.baseURL)
}

func (s *Session) login(element *colly.HTMLElement) {
	data := map[string]string{
		"j_username": s.username,
		"j_password": s.password,
	}

	if err := element.Request.Post(element.Attr("action"), data); err != nil {
		s.err = err
	}
}

func (s *Session) loadStudentsFromJSON(response *colly.Response) {
	if s.err != nil {
		return
	}

	var students []*Student
	if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&students); err != nil {
		s.err = err
		return
	}

	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	s.Students = students
}

func (s *Session) loadCoursesFromJSON(student *Student) func(*colly.Response) {
	return func(response *colly.Response) {
		if s.err != nil {
			return
		}

		var courses []*Course
		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&courses); err != nil {
			s.err = err
			return
		}

		sort.Slice(courses, func(i, j int) bool { return courses[i].ID < courses[j].ID })
		student.Courses = courses
	}
}

func (s *Session) loadAssignmentsFromJSON(course *Course) func(*colly.Response) {
	return func(response *colly.Response) {
		if s.err != nil {
			return
		}

		var helper assignmentResponseHelper
		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&helper); err != nil {
			s.err = err
			return
		}

		course.Assignments = helper.Assignments
	}
}

func (s *Session) getAndUpdate(link string, loader func(*colly.Response)) {
	if s.err != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Visit(link); err != nil {
		s.err = err
	}
}

func (s *Session) postAndUpdate(link string, data map[string]string, loader func(*colly.Response)) {
	if s.err != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Post(link, data); err != nil {
		s.err = err
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
}

func ts() int64 { return time.Now().Unix() }

var htmlHelpers = template.FuncMap{
	"htmlsafe": func(s string) template.HTML { return template.HTML(s) },
	"tolower":  strings.ToLower,
	"timenow":  func() string { return time.Now().Format(time.RFC3339) },
}

const (
	headerTemplate = `<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Ignitia Report {{ range .Students }}| {{ .DisplayName | htmlsafe }}{{ end }}</title>
  <link href="/static/style.css" rel="stylesheet" type="text/css">
  <script async src="/static/page.js" type="text/javascript"></script>
</head>
<body>
`
	footerTemplate = `
  <footer>
    <p>As of {{ timenow }}</p>
  </footer>
</body>
</html>
`
	reportTemplate = `
<div class="report" data-num-students="{{ len .Students }}">
  {{- range .Students }}{{ $student_id := .ID }}
  <section id="student_{{ $student_id }}" class="student" data-num-courses="{{ len .Courses }}" data-num-courses-incomplete="{{ .IncompleteCourses }}">
    <h2>{{ .DisplayName | htmlsafe }}</h2>
    {{- range .Courses }}{{ $course_id := .ID }}
    <section id="course_{{ $student_id }}_{{ $course_id }}" class="course" data-num-assignments="{{ len .Assignments }}" data-num-assignments-incomplete="{{ .IncompleteAssignments }}">
      <h3>{{ .Title }}</h3>
      {{- range .Assignments }}{{ $assignment_id := .ID }}
        <section id="assignment_{{ $student_id }}_{{ $course_id }}_{{ $assignment_id }}" class="assignment {{ if .IsIncomplete }}in{{ end }}complete{{ if .IsDue }} due{{ end }}{{ if .IsOverdue }} overdue{{ end }}{{ if .IsFuture }} future{{ end }}{{ if .IsPast }} past{{ end }} type_{{ .Type | tolower }}">
          <h4>Unit {{ .Unit }}</h4>
          <h4>{{ .Title }}</h4>
          <h5>{{ .Type }}</h5>
          <h5>{{ .Status }}</h5>
          <dl>
            <dt>Due</dt>
            <dd>{{ .Due }}</dd>
            {{- if ne .Completed "" }}

            <dt>Completed</dt>
            <dd>{{ .Completed }}</dd>
            {{- end }}
            <dt>Progress</dt>
            <dd>{{ .Progress }}%</dd>
            {{- if ne .Score 0 }}

            <dt>Score</dt>
            <dd>{{ .Score }}%</dd>
            {{- end }}
          </dl>
        </section>
      {{- end }}
    </section>
    {{- end }}
  </section>
  {{- end }}
</div>
`
)
