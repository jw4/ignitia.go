package main

import (
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/jw4/ignitia.go/pkg/collect"
	"github.com/jw4/ignitia.go/pkg/model"
	"github.com/jw4/ignitia.go/pkg/web"

	_ "github.com/jw4/ignitia.go/pkg/model/persistence"
)

var version = "dev"

func main() {
	ses := model.New(os.Getenv("IGNITIA_DB"))
	if ses == nil {
		fmt.Fprintf(os.Stderr, "unable to open model for %q\n", os.Getenv("IGNITIA_DB"))
		doHelp()
		os.Exit(1)
	}

	opts := []web.Option{
		web.Assets(os.Getenv("PUBLIC_ASSETS")),
		web.Templates(os.Getenv("TEMPLATES")),
	}
	webSession := web.NewSession(ses, opts...)

	action := "help"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "serve":
		doServe(webSession)
	case "html":
		doHTML(webSession)
	case "due":
		doPrint(webSession, isDue)
	case "overdue":
		doPrint(webSession, isOverdue)
	case "snapshot":
		doSnapshot(ses,
			collect.NewSession(
				os.Getenv("IGNITIA_BASE_URL"),
				os.Getenv("IGNITIA_USERNAME"),
				os.Getenv("IGNITIA_PASSWORD")))
	default:
		doHelp()
	}
}

func doHelp() { fmt.Fprint(os.Stderr, helpText) }

func doServe(session *web.Session) {
	bind := os.Getenv("BIND")
	fmt.Fprintf(os.Stderr, "Version: %s\n", version)
	fmt.Fprintf(os.Stderr, "Serving on %s\n", bind)

	if err := http.ListenAndServe(bind, session); err != nil {
		fmt.Fprintf(os.Stderr, "error serving: %v\n", err)
		os.Exit(-1)
	}
}

func doHTML(session *web.Session) {
	session.DebugWriter = ioutil.Discard

	if err := session.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "error refreshing: %v\n", err)
		os.Exit(-1)
	}

	if err := session.RenderHTML(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering HTML: %v", err)
		os.Exit(-1)
	}
}

func doPrint(session *web.Session, with func(model.Assignment) bool) {
	session.DebugWriter = ioutil.Discard
	session.DebugWriter = os.Stderr

	if err := session.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "error refreshing: %v\n", err)
		os.Exit(-1)
	}

	print(filter(session.Students, with), os.Stdout)
}

func doSnapshot(writer model.Write, reader model.Read) {
	if err := writer.Save(reader); err != nil {
		fmt.Fprintf(os.Stderr, "error snapshotting: %v\n", err)
		os.Exit(-1)
	}
}

func print(students []model.Student, out io.Writer) {
	for _, student := range students {
		if len(student.Courses) == 0 {
			continue
		}

		fmt.Fprintf(out, "\nStudent: %s\n", html.UnescapeString(student.DisplayName))

		for _, course := range student.Courses {
			if len(course.Assignments) == 0 {
				continue
			}

			fmt.Fprintf(out, "\n  Course: %s; %d assignments\n", course.Title, len(course.Assignments))

			for _, assignment := range course.Assignments {
				fmt.Fprintf(out, "    Assignment: %s\n", assignment.String())
			}
		}
	}
}

func isDue(a model.Assignment) bool     { return a.IsDue() }
func isOverdue(a model.Assignment) bool { return a.IsOverdue() }

func filter(students []model.Student, with func(model.Assignment) bool) []model.Student {
	var filtered []model.Student
	for _, student := range students {
		var filteredCourses []model.Course

		for _, course := range student.Courses {
			var filteredAssignments []model.Assignment

			for _, assignment := range course.Assignments {
				if with(assignment) {
					filteredAssignments = append(filteredAssignments, assignment)
				}
			}

			if len(filteredAssignments) > 0 {
				filteredCourses = append(filteredCourses, model.Course{
					ID:          course.ID,
					Title:       course.Title,
					Assignments: filteredAssignments,
				})
			}
		}

		if len(filteredCourses) > 0 {
			filtered = append(filtered, model.Student{
				ID:          student.ID,
				DisplayName: student.DisplayName,
				Courses:     filteredCourses,
			})
		}
	}

	return filtered
}

const helpText = `
options:

  serve      serve web page
  html       render report in HTML
  due        print due assignments
  overdue    print overdue assignments
  snapshot   update sqlite db
  help       display this help

`
