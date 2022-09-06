package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jw4/ignitia.go/pkg/model"
)

var (
	logRequestResponse = false
	logJSON            = false
)

type Session struct {
	errSession error

	collector *colly.Collector
	logger    *log.Logger

	baseURL  string
	username string
	password string
}

func NewSession(baseURL, username, password string) *Session {
	logger := log.New(os.Stderr, " [session] ", log.LstdFlags)
	return &Session{
		baseURL:  baseURL,
		username: username,
		password: password,
		logger:   logger,
	}
}

func (s *Session) Error() error { return s.errSession }
func (s *Session) Reset()       { s.collector = nil }

func (s *Session) Data() model.Data {
	data := model.Data{AsOf: time.Now(), Students: map[int]*model.Student{}}

	for _, student := range s.Students() {
		student.Courses = map[int]*model.Course{}

		for _, course := range s.Courses(student) {
			course.Assignments = map[int]*model.Assignment{}

			for _, assignment := range s.Assignments(student, course) {
				course.Assignments[assignment.ID] = assignment
			}

			student.Courses[course.ID] = course
		}

		data.Students[student.ID] = student
	}

	return data
}

func (s *Session) Students() []*model.Student {
	if err := s.init(); err != nil {
		return nil
	}

	var students []*model.Student
	loadStudentsFromJSON := func(response *colly.Response) {
		if !(assertOK(response) && assertJSON(response)) {
			return
		}

		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&students); err != nil {
			s.errSession = err
			return
		}

		sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	}

	s.getAndUpdate(fmt.Sprintf("%s/owsoo/parent/populateStudents?_=%d", s.baseURL, ts()), loadStudentsFromJSON)

	return students
}

func (s *Session) Courses(student *model.Student) []*model.Course {
	if err := s.init(); err != nil {
		return nil
	}

	var courses []*model.Course
	loadCoursesFromJSON := func(response *colly.Response) {
		if !(assertOK(response) && assertJSON(response)) {
			return
		}

		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&courses); err != nil {
			s.errSession = err
			return
		}

		for _, course := range courses {
			course.StudentID = student.ID
		}

		sort.Slice(courses, func(i, j int) bool { return courses[i].ID < courses[j].ID })
	}

	s.getAndUpdate(
		fmt.Sprintf("%s/owsoo/parent/populateCourses?student_id=%d&_=%d", s.baseURL, student.ID, ts()),
		loadCoursesFromJSON)

	return courses
}

func (s *Session) Assignments(student *model.Student, course *model.Course) []*model.Assignment {
	if err := s.init(); err != nil {
		return nil
	}

	var helper assignmentResponseHelper

	loadAssignmentsFromJSON := func(response *colly.Response) {
		if s.errSession != nil {
			return
		}

		if !(assertOK(response) && assertJSON(response)) {
			return
		}

		if err := json.NewDecoder(bytes.NewReader(response.Body)).Decode(&helper); err != nil {
			s.errSession = err
			return
		}

		for _, assignment := range helper.Assignments {
			assignment.StudentID = student.ID
			assignment.CourseID = course.ID
		}
	}

	data := map[string]string{
		"student_id":    fmt.Sprintf("%d", student.ID),
		"enrollment_id": fmt.Sprintf("%d", course.ID),
		"nd":            fmt.Sprintf("%d", ts()),
		"rows":          "1000",
		"page":          "1",
	}

	s.postAndUpdate(
		fmt.Sprintf("%s/owsoo/parent/listAssignmentsByCourse", s.baseURL),
		data, loadAssignmentsFromJSON)

	return helper.Assignments
}

func (s *Session) init() error {
	if s.collector != nil {
		return nil
	}

	s.collector = colly.NewCollector(
		colly.UserAgent("report-generator v0.0.1 (johnweldon4@gmail.com)"),
	)

	s.collector.SetRequestTimeout(30 * time.Second)

	s.collector.OnRequest(s.logRequest)
	s.collector.OnResponse(s.logResponse)

	s.collector.OnHTML("#loginForm", s.login)
	s.collector.OnHTML(".login-error", s.loginError)

	if err := s.collector.Visit(s.baseURL); err != nil {
		return err
	}

	return nil
}

func (s *Session) login(element *colly.HTMLElement) {
	data := map[string]string{
		"j_username": s.username,
		"j_password": s.password,
	}

	if err := element.Request.Post(element.Attr("action"), data); err != nil {
		s.errSession = err
	}
}

func (s *Session) loginError(element *colly.HTMLElement) {
	s.errSession = fmt.Errorf("error logging in: %s", element.Text)
}

func (s *Session) getAndUpdate(link string, loader func(*colly.Response)) {
	if s.errSession != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Visit(link); err != nil {
		s.errSession = err
	}
}

func (s *Session) postAndUpdate(link string, data map[string]string, loader func(*colly.Response)) {
	if s.errSession != nil {
		return
	}

	clone := s.collector.Clone()

	clone.OnRequest(s.logRequest)
	clone.OnResponse(s.logResponse)

	clone.OnResponse(loader)

	if err := clone.Post(link, data); err != nil {
		s.errSession = err
	}
}

func (s *Session) logRequest(request *colly.Request) {
	if !logRequestResponse || s.logger == nil {
		return
	}

	s.logger.Printf("%d %s %s\n", request.ID, request.Method, request.URL)
}

func (s *Session) logResponse(response *colly.Response) {
	if !logRequestResponse || s.logger == nil {
		return
	}

	s.logger.Printf("response %d %d %s\n",
		response.Request.ID,
		response.StatusCode,
		http.StatusText(response.StatusCode),
	)

	if !shouldLogBody(response) {
		return
	}

	s.logger.Printf("response body:\n---\n%s\n---\n", string(response.Body))
}

func shouldLogBody(response *colly.Response) bool {
	if response.StatusCode >= http.StatusBadRequest {
		return true
	}

	ct, _, err := mime.ParseMediaType(response.Headers.Get("Content-Type"))
	if err != nil {
		return true
	}

	switch ct {
	case "application/json":
		return logJSON
	}

	return false
}

func assertOK(r *colly.Response) bool {
	if r.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(os.Stderr, "Unexpected error: %d (%v)\n---\n%s\n---\n", r.StatusCode, r.Headers, string(r.Body))
		return false
	}
	return true
}

func assertJSON(response *colly.Response) bool {
	mediaType, _, err := mime.ParseMediaType(response.Headers.Get("content-type"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parsing Media Type %q: %v\n", response.Headers.Get("content-type"), err)
		return false
	}

	if mediaType != "application/json" {
		fmt.Fprintf(os.Stderr, "Media Type %q\n---\n%s\n---\n", mediaType, string(response.Body))
		return false
	}

	return true
}

func ts() int64 { return time.Now().Unix() }
