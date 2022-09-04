package persistence

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/jw4/ignitia.go/pkg/model"
)

func init() {
	model.RegisterHandler("sqlite", func(s string) (func(string) model.Full, bool) {
		if strings.HasSuffix(s, ".db") {
			return NewSQLiteModel, true
		}

		return nil, false
	})
}

type SQLiteModel struct {
	conn     *sql.DB
	modelErr error
	dbPath   string
}

func NewSQLiteModel(conn string) model.Full {
	m := &SQLiteModel{dbPath: conn}
	m.Reset()
	return m
}

func (m *SQLiteModel) Save(reader model.Read) error {
	students := reader.Students()

	if err := m.SaveStudents(students); err != nil {
		return fmt.Errorf("saving students: %w", err)
	}

	for _, student := range students {
		courses := reader.Courses(student)

		if err := m.SaveCourses(student, courses); err != nil {
			return fmt.Errorf("saving courses: %w", err)
		}

		for _, course := range courses {
			assignments := reader.Assignments(student, course)

			if err := m.SaveAssignments(student, course, assignments); err != nil {
				return fmt.Errorf("saving assignments: %w", err)
			}
		}
	}

	return nil
}

func (m *SQLiteModel) SaveStudents(students []model.Student) error {
	if err := m.Reset(); err != nil {
		return err
	}

	stmt, err := m.conn.Prepare(saveStudentSQL)
	if err != nil {
		return err
	}

	tx, err := m.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, student := range students {
		if _, err = stmt.Exec(student.ID, student.DisplayName); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *SQLiteModel) SaveCourses(student model.Student, courses []model.Course) error {
	if err := m.Reset(); err != nil {
		return err
	}

	saveCourseStmt, err := m.conn.Prepare(saveCourseSQL)
	if err != nil {
		return err
	}

	saveStudentCourseStmt, err := m.conn.Prepare(saveStudentCourseSQL)
	if err != nil {
		return err
	}

	tx, err := m.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, course := range courses {
		if _, err = saveCourseStmt.Exec(course.ID, course.Title); err != nil {
			return err
		}

		if _, err = saveStudentCourseStmt.Exec(student.ID, course.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *SQLiteModel) SaveAssignments(student model.Student, course model.Course, assignments []model.Assignment) error {
	if err := m.Reset(); err != nil {
		return err
	}

	saveAssignmentStmt, err := m.conn.Prepare(saveAssignmentSQL)
	if err != nil {
		return err
	}

	saveHistoryStmt, err := m.conn.Prepare(saveAssignmentHistorySQL)
	if err != nil {
		return err
	}

	tx, err := m.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, assignment := range assignments {
		if _, err = saveAssignmentStmt.Exec(assignment.ID, course.ID, assignment.Unit, assignment.Title, assignment.Type); err != nil {
			return err
		}

		due, completed := assignment.Due, assignment.Completed
		if len(due) > 0 {
			due = assignment.DueDate().Format("2006-01-02")
		}

		if len(completed) > 0 {
			completed = assignment.CompleteDate().Format("2006-01-02")
		}

		if _, err = saveHistoryStmt.Exec(student.ID, assignment.ID, assignment.Progress, due, completed, assignment.Score, assignment.Status); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (m *SQLiteModel) Close() error {
	err := m.conn.Close()
	if err != nil {
		log.Printf("warning while closing connection: %v", err)
	}

	m.conn = nil

	return err
}

func (m *SQLiteModel) Reset() error {
	if m.conn != nil {
		if m.modelErr == nil {
			return nil
		}

		_ = m.Close()
	}

	m.conn, m.modelErr = sqliteOpen(m.dbPath)

	return m.modelErr
}

func (m *SQLiteModel) Error() error { return m.modelErr }

func (m *SQLiteModel) Students() []model.Student {
	if err := m.Reset(); err != nil {
		log.Printf("model error: %v", err)
		return nil
	}

	rows, err := m.conn.Query(selectStudentsSQL)
	if err != nil {
		m.modelErr = err
		return nil
	}
	defer rows.Close()

	var students []model.Student

	for rows.Next() {
		var student model.Student
		if err = rows.Scan(&student.ID, &student.DisplayName); err != nil {
			m.modelErr = err
			return students
		}
		students = append(students, student)
	}

	m.modelErr = rows.Err()

	return students
}

func (m *SQLiteModel) Courses(student model.Student) []model.Course {
	if err := m.Reset(); err != nil {
		log.Printf("model error: %v", err)
		return nil
	}

	rows, err := m.conn.Query(selectCoursesSQL, student.ID)
	if err != nil {
		m.modelErr = err
		return nil
	}
	defer rows.Close()

	var courses []model.Course

	for rows.Next() {
		var course model.Course
		if err = rows.Scan(&course.ID, &course.Title); err != nil {
			m.modelErr = err
			return courses
		}

		courses = append(courses, course)
	}

	m.modelErr = rows.Err()

	return courses
}

func (m *SQLiteModel) Assignments(student model.Student, course model.Course) []model.Assignment {
	if err := m.Reset(); err != nil {
		log.Printf("model error: %v", err)
		return nil
	}

	rows, err := m.conn.Query(selectAssignmentsSQL, student.ID, course.ID)
	if err != nil {
		m.modelErr = err
		return nil
	}
	defer rows.Close()

	var assignments []model.Assignment

	for rows.Next() {
		var assignment model.Assignment
		if err = rows.Scan(
			&assignment.ID,
			&assignment.Unit,
			&assignment.Title,
			&assignment.Type,
			&assignment.Progress,
			&assignment.Due,
			&assignment.Completed,
			&assignment.Score,
			&assignment.Status,
			&assignment.AsOf,
		); err != nil {
			m.modelErr = err
			return assignments
		}

		assignments = append(assignments, assignment)
	}

	m.modelErr = rows.Err()

	return assignments
}

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
	//go:embed schema.sql
	schema string

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
      status,
      as_of
    FROM
      student_assignments
    WHERE
      student_id = ?
      AND course_id = ?
    ;
    `
)
