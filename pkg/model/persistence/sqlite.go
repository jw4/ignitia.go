package persistence

import (
	"database/sql"
	"fmt"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/jw4/ignitia.go/pkg/model"
)

type Collector interface {
	Students() []model.Student
	Courses(model.Student) []model.Course
	Assignments(model.Student, model.Course) []model.Assignment
}

func SQLiteSnapshot(path string, collector Collector) error {
	p := NewModel(path)
	err := p.Error()
	if err != nil {
		return fmt.Errorf("initializing model: %w", err)
	}

	students := collector.Students()

	if err = p.SaveStudents(students); err != nil {
		return fmt.Errorf("saving students: %w", err)
	}

	for _, student := range students {
		courses := collector.Courses(student)

		if err = p.SaveCourses(student, courses); err != nil {
			return fmt.Errorf("saving courses: %w", err)
		}

		for _, course := range courses {
			assignments := collector.Assignments(student, course)

			if err = p.SaveAssignments(student, course, assignments); err != nil {
				return fmt.Errorf("saving assignments: %w", err)
			}
		}
	}

	return nil
}

//go:embed schema.sql
var schema string

func sqliteOpen(name string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", name)
	if err != nil {
		return nil, err
	}

	if _, err = db.Exec(schema); err != nil {
		return nil, err
	}

	return db, nil
}

var (
	saveStudentSQL       = `INSERT INTO student (id, name) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name;`
	saveCourseSQL        = `INSERT INTO course (id, title) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET title=excluded.title;`
	saveStudentCourseSQL = `INSERT INTO student_courses (student_id, course_id) VALUES (?, ?) ON CONFLICT DO NOTHING;`
	saveAssignmentSQL    = `
INSERT INTO assignment (
  id,
  course_id,
  unit,
  title,
  assignment_type
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  course_id = excluded.course_id,
  unit = excluded.unit,
  title = excluded.title,
  assignment_type = excluded.assignment_type
;
`
	saveAssignmentHistorySQL = `
INSERT INTO assignment_history (
  student_id,
  assignment_id,
  as_of,
  progress,
  due,
  completed,
  score,
  status
) VALUES (?, ?, datetime(), ?, ?, ?, ?, ?)
ON CONFLICT DO UPDATE SET
  due = excluded.due,
  completed = excluded.completed,
  score = excluded.score,
  status = excluded.status
;
`

	selectStudentsSQL = `SELECT id, name FROM student;`

	selectCoursesSQL = `
SELECT
  c.id,
  c.title
FROM
  student_courses sc
  JOIN course c ON c.id = sc.course_id
WHERE
  sc.student_id = ?
;
`

	selectAssignmentsSQL = `
SELECT
  id,
  unit,
  title,
  assignment_type,
  progress,
  due,
  completed,
  score,
  status
FROM
  student_assignments
WHERE
  student_id = ?
  AND course_id = ?
;
`
)
