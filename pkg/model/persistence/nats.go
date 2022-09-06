package persistence

import (
	"encoding/json"
	"strings"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/nats-io/nats.go"

	"github.com/jw4/ignitia.go/pkg/model"
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
	connURL string

	conn *nats.Conn
	js   nats.JetStreamContext

	ignitiaBucket nats.KeyValue

	data model.Data
}

func (n *NATSModel) refresh() error {
	var (
		err     error
		entry   nats.KeyValueEntry
		working model.Data
	)

	if n.ignitiaBucket == nil {
		if n.ignitiaBucket, err = n.openOrCreate(ignitiaBucket); err != nil {
			return err
		}
	}

	if entry, err = n.ignitiaBucket.Get(ignitiaKey); err != nil {
		return err
	}

	if err = json.Unmarshal(entry.Value(), &working); err != nil {
		return err
	}

	n.data = working

	return nil
}

func (n *NATSModel) Data() model.Data {
	if err := n.refresh(); err != nil {
		return model.Data{Errors: []error{err}, AsOf: time.Now()}
	}

	return n.data
}

func (n *NATSModel) Students() []model.Student {
	if err := n.refresh(); err != nil {
		return nil
	}

	var students []model.Student
	for _, student := range n.data.Students {
		students = append(students, *student)
	}

	return students
}

func (n *NATSModel) Courses(student model.Student) []model.Course {
	if err := n.refresh(); err != nil {
		return nil
	}

	s, ok := n.data.Students[student.ID]
	if !ok {
		return nil
	}

	var courses []model.Course
	for _, course := range s.Courses {
		courses = append(courses, *course)
	}

	return courses
}

func (n *NATSModel) Assignments(student model.Student, course model.Course) []model.Assignment {
	if err := n.refresh(); err != nil {
		return nil
	}

	c, ok := n.data.Students[student.ID].Courses[course.ID]
	if !ok {
		return nil
	}

	var assignments []model.Assignment
	for _, assignment := range c.Assignments {
		assignments = append(assignments, *assignment)
	}

	return assignments
}

func (n *NATSModel) Save(reader model.Read) error {
	var (
		err  error
		data []byte
	)

	if n.ignitiaBucket == nil {
		if n.ignitiaBucket, err = n.openOrCreate(ignitiaBucket); err != nil {
			return err
		}
	}

	if data, err = json.Marshal(reader.Data()); err != nil {
		return err
	}

	_, err = n.ignitiaBucket.Put(ignitiaKey, data)
	return err
}

func (n *NATSModel) Close() error {
	n.conn.Close()
	n.conn = nil
	n.js = nil

	return nil
}

func (n *NATSModel) Reset() error {
	var err error

	if n.conn != nil {
		if err = n.Close(); err != nil {
			return err
		}
	}

	n.conn, err = nats.Connect(n.connURL)
	if err == nil {
		n.js, err = n.conn.JetStream()
	}

	n.data = model.Data{AsOf: time.Now()}

	return err
}

func (n *NATSModel) Error() error {
	if len(n.data.Errors) > 0 {
		return multierror.Append(nil, n.data.Errors...)
	}

	return nil
}

const (
	ignitiaBucket = "ignitia"
	ignitiaKey    = "simeon"
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
