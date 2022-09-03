package persistence

import (
	"database/sql"
	"log"

	"github.com/jw4/ignitia.go/pkg/model"
)

type Model struct {
	conn     *sql.DB
	modelErr error
	dbPath   string
}

func NewModel(dbPath string) *Model {
	m := &Model{dbPath: dbPath}
	m.Reset()
	return m
}

func (m *Model) SaveStudents(students []model.Student) error {
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

func (m *Model) SaveCourses(student model.Student, courses []model.Course) error {
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

func (m *Model) SaveAssignments(student model.Student, course model.Course, assignments []model.Assignment) error {
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

func (m *Model) Close() error {
	err := m.conn.Close()
	if err != nil {
		log.Printf("warning while closing connection: %v", err)
	}

	m.conn = nil

	return err
}

func (m *Model) Reset() error {
	if m.conn != nil {
		if m.modelErr == nil {
			return nil
		}

		_ = m.Close()
	}

	m.conn, m.modelErr = sqliteOpen(m.dbPath)

	return m.modelErr
}

func (m *Model) Error() error { return m.modelErr }

func (m *Model) Students() []model.Student {
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

func (m *Model) Courses(student model.Student) []model.Course {
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

func (m *Model) Assignments(student model.Student, course model.Course) []model.Assignment {
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
