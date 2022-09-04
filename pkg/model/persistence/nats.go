package persistence

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jw4/ignitia.go/pkg/model"
	"github.com/nats-io/nats.go"
)

func init() {
	model.RegisterHandler("nats", func(s string) (func(string) model.Full, bool) {
		if strings.HasPrefix(s, "nats://") {
			return NewNATSModel, true
		}

		return nil, false
	})
}

func NewNATSModel(conn string) model.Full {
	return &NATSModel{connURL: conn}
}

type NATSModel struct {
	modelErr error
	connURL  string

	conn *nats.Conn
	js   nats.JetStreamContext

	studentBucket    nats.KeyValue
	courseBucket     nats.KeyValue
	assignmentBucket nats.KeyValue
}

func (n *NATSModel) Students() []model.Student {
	var (
		err      error
		students []model.Student
	)

	if n.studentBucket == nil {
		if n.studentBucket, err = n.openOrCreate(studentBucket); err != nil {
			n.modelErr = err
			return nil
		}
	}

	studentIDs, err := n.studentBucket.Keys()
	if err != nil {
		n.modelErr = err
		return nil
	}

	for _, id := range studentIDs {
		v, err := n.studentBucket.Get(id)
		if err != nil {
			n.modelErr = err
			return nil
		}

		var student model.Student
		if err = unmarshalStudent(v.Value(), &student); err != nil {
			n.modelErr = fmt.Errorf("unmarshaling student (%s), %w", v.Value(), err)
			return nil
		}

		student.Courses = n.Courses(student)

		students = append(students, student)
	}

	return students
}

func (n *NATSModel) Courses(student model.Student) []model.Course {
	var (
		err     error
		courses []model.Course
	)

	if n.courseBucket == nil {
		if n.courseBucket, err = n.openOrCreate(courseBucket); err != nil {
			n.modelErr = err
			return nil
		}
	}

	courseIDs, err := n.courseBucket.Keys()
	if err != nil {
		n.modelErr = err
		return nil
	}

	idPrefix := fmt.Sprintf("%d.", student.ID)

	for _, id := range courseIDs {
		if !strings.HasPrefix(id, idPrefix) {
			continue
		}

		v, err := n.courseBucket.Get(id)
		if err != nil {
			n.modelErr = err
			return nil
		}

		var course model.Course
		if err = unmarshalCourse(v.Value(), &course); err != nil {
			n.modelErr = fmt.Errorf("unmarshaling course (%s), %w", v.Value(), err)
			return nil
		}

		course.Assignments = n.Assignments(student, course)

		courses = append(courses, course)
	}

	return courses
}

func (n *NATSModel) Assignments(student model.Student, course model.Course) []model.Assignment {
	var (
		err         error
		assignments []model.Assignment
	)

	if n.assignmentBucket == nil {
		if n.assignmentBucket, err = n.openOrCreate(assignmentBucket); err != nil {
			n.modelErr = err
			return nil
		}
	}

	assignmentIDs, err := n.assignmentBucket.Keys()
	if err != nil {
		n.modelErr = err
		return nil
	}

	idPrefix := fmt.Sprintf("%d.%d.", student.ID, course.ID)

	for _, id := range assignmentIDs {
		if !strings.HasPrefix(id, idPrefix) {
			continue
		}

		v, err := n.assignmentBucket.Get(id)
		if err != nil {
			n.modelErr = err
			return nil
		}

		var assignment model.Assignment
		if err = unmarshalAssignment(v.Value(), &assignment); err != nil {
			n.modelErr = fmt.Errorf("unmarshaling assignment (%s), %w", v.Value(), err)
			return nil
		}

		assignments = append(assignments, assignment)
	}

	return assignments
}

func (n *NATSModel) Save(reader model.Read) error {
	var err error

	students := reader.Students()

	if err = n.SaveStudents(students); err != nil {
		return fmt.Errorf("saving students: %w", err)
	}

	for _, student := range students {
		courses := reader.Courses(student)

		if err = n.SaveCourses(student, courses); err != nil {
			return fmt.Errorf("saving courses: %w", err)
		}

		for _, course := range courses {
			assignments := reader.Assignments(student, course)

			if err = n.SaveAssignments(student, course, assignments); err != nil {
				return fmt.Errorf("saving assignments: %w", err)
			}
		}
	}

	return nil
}

func (n *NATSModel) SaveStudents(students []model.Student) error {
	var (
		err  error
		data []byte
	)

	if n.studentBucket == nil {
		if n.studentBucket, err = n.openOrCreate(studentBucket); err != nil {
			return err
		}
	}

	for _, student := range students {
		if data, err = marshal(student); err != nil {
			return err
		}

		if err = n.save(n.studentBucket, fmt.Sprintf("%d", student.ID), data); err != nil {
			return err
		}
	}

	return n.modelErr
}

func (n *NATSModel) SaveCourses(student model.Student, courses []model.Course) error {
	var (
		err  error
		data []byte
	)

	if n.courseBucket == nil {
		if n.courseBucket, err = n.openOrCreate(courseBucket); err != nil {
			return err
		}
	}

	for _, course := range courses {
		if data, err = marshal(course); err != nil {
			return err
		}

		if err = n.save(n.courseBucket, fmt.Sprintf("%d.%d", student.ID, course.ID), data); err != nil {
			return err
		}
	}

	return n.modelErr
}

func (n *NATSModel) SaveAssignments(student model.Student, course model.Course, assignments []model.Assignment) error {
	var (
		err  error
		data []byte
	)

	if n.assignmentBucket == nil {
		if n.assignmentBucket, err = n.openOrCreate(assignmentBucket); err != nil {
			return err
		}
	}

	prefix := fmt.Sprintf("%d.%d.", student.ID, course.ID)
	for _, assignment := range assignments {
		if data, err = marshal(assignment); err != nil {
			return err
		}

		if err = n.save(n.assignmentBucket, fmt.Sprintf("%s%d", prefix, assignment.ID), data); err != nil {
			return err
		}
	}

	return n.modelErr
}

func (n *NATSModel) Close() error {
	n.conn.Close()
	n.conn = nil
	n.js = nil

	return nil
}

func (n *NATSModel) Reset() error {
	if n.conn != nil {
		if n.modelErr == nil {
			return nil
		}

		_ = n.Close()
	}

	n.conn, n.modelErr = nats.Connect(n.connURL)
	if n.modelErr == nil {
		n.js, n.modelErr = n.conn.JetStream()
	}

	return n.modelErr
}

func (n *NATSModel) Error() error { return n.modelErr }

const (
	studentBucket    = "ignitia_students"
	courseBucket     = "ignitia_courses"
	assignmentBucket = "ignitia_assignments"
)

func (n *NATSModel) openOrCreate(bucket string) (nats.KeyValue, error) {
	if err := n.Reset(); err != nil {
		return nil, err
	}

	kv, err := n.js.KeyValue(bucket)
	if err != nil {
		kv, err = n.js.CreateKeyValue(&nats.KeyValueConfig{Bucket: bucket})
		if err != nil {
			return nil, err
		}
	}

	return kv, nil
}

func (n *NATSModel) save(bucket nats.KeyValue, key string, data []byte) error {
	_, n.modelErr = bucket.Put(key, data)
	return n.modelErr
}

func (n *NATSModel) get(bucket nats.KeyValue, key string) ([]byte, error) {
	e, err := bucket.Get(key)
	if err != nil {
		return nil, err
	}

	return e.Value(), nil
}

func marshal(val any) ([]byte, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func unmarshalStudent(data []byte, into *model.Student) error { return json.Unmarshal(data, into) }
func unmarshalCourse(data []byte, into *model.Course) error   { return json.Unmarshal(data, into) }
func unmarshalAssignment(data []byte, into *model.Assignment) error {
	return json.Unmarshal(data, into)
}
