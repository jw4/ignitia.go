package main

import (
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/jw4/ignitia.go"
)

var version = "dev"

func main() {
	opts := []ignitia.Option{
		ignitia.BaseURL(os.Getenv("IGNITIA_BASE_URL")),
		ignitia.Credentials(os.Getenv("IGNITIA_USERNAME"), os.Getenv("IGNITIA_PASSWORD")),
		ignitia.Assets(os.Getenv("PUBLIC_ASSETS")),
		ignitia.Templates(os.Getenv("TEMPLATES")),
	}

	action := "help"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "serve":
		doServe(ignitia.NewSession(opts...))
	case "html":
		doHTML(ignitia.NewSession(opts...))
	case "print":
		doPrint(ignitia.NewSession(opts...))
	default:
		doHelp()
	}
}

func doHelp() { fmt.Fprint(os.Stderr, helpText) }

func doServe(session *ignitia.Session) {
	bind := os.Getenv("BIND")
	fmt.Fprintf(os.Stderr, "Version: %s\n", version)
	fmt.Fprintf(os.Stderr, "Serving on %s\n", bind)

	if err := http.ListenAndServe(bind, session); err != nil {
		fmt.Fprintf(os.Stderr, "error serving: %v\n", err)
		os.Exit(-1)
	}
}

func doHTML(session *ignitia.Session) {
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

func doPrint(session *ignitia.Session) {
	session.DebugWriter = ioutil.Discard

	if err := session.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "error refreshing: %v\n", err)
		os.Exit(-1)
	}

	printDue(session, os.Stdout)
}

func printDue(session *ignitia.Session, out io.Writer) {
	for _, student := range session.Students {
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
				if assignment.IsDue() {
					fmt.Fprintf(out, "    Assignment: %s\n", assignment)
				}
			}
		}
	}
}

const helpText = `
options:

  serve		serve web page

  html  	render report in HTML

  print 	render report in plain text

  help		display this help

`
