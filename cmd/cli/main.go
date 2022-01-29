package main

import (
	"log"
	"os"

	"github.com/jw4/ignitia.go/pkg/collect"
)

func main() {
	baseURL := os.Getenv("IGNITIA_BASE_URL")
	username := os.Getenv("IGNITIA_USERNAME")
	password := os.Getenv("IGNITIA_PASSWORD")
	ses := collect.NewSession(baseURL, username, password)
	students := ses.Students()

	for _, student := range students {
		log.Printf("Student: %+v\n", student)
		courses := ses.Courses(student)
		for _, course := range courses {
			log.Printf("  Course: %+v\n", course)
			assignments := ses.Assignments(student, course)
			for _, assignment := range assignments {
				log.Printf("    Assignment: %+v\n", assignment)
			}
		}
	}
}
