package collect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/jw4/ignitia.go/pkg/model"
)

var (
	ErrValidation = errors.New("validation error")
	ErrMarshal    = errors.New("json marshaling error")
)

func ToAssignment(raw map[string]interface{}) (*model.Assignment, error) { // nolint: funlen,cyclop
	id, ok := raw["id"].(float64) // nolint: varnamelen
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for cell id: got %T (%v), expected number [%w]",
			raw["id"], raw["id"], ErrValidation)
	}

	assignment := &model.Assignment{ID: int(id)} // nolint: exhaustivestruct

	items, ok := raw["cell"].([]interface{})
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for cell values: got %T (%v), expected []interface{} [%w]",
			raw["cell"], raw["cell"], ErrValidation)
	}

	const columns = 9
	if len(items) != columns {
		return nil, fmt.Errorf(
			"unexpected length of items, got %d (%v), expected %d [%w]",
			len(items), items, columns, ErrValidation)
	}

	_, ok = items[0].(float64)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for ID (items[0]): got %T (%v), expected number [%w]",
			items[0], items[0], ErrValidation)
	}

	unit, ok := items[1].(float64)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Unit (items[1]): got %T (%v), expected number [%w]",
			items[1], items[1], ErrValidation)
	}

	assignment.Unit = int(unit)

	title, ok := items[2].(string)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Title (items[2]): got %T (%v), expected string [%w]",
			items[2], items[2], ErrValidation)
	}

	assignment.Title = title

	typ, ok := items[3].(string)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Type (items[3]): got %T (%v), expected string [%w]",
			items[3], items[3], ErrValidation)
	}

	assignment.Type = typ

	progress, ok := items[4].(float64)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Progress (items[4]): got %T (%v), expected number [%w]",
			items[4], items[4], ErrValidation)
	}

	assignment.Progress = int(progress)

	switch due := items[5].(type) {
	case string:
		assignment.Due = due
	case nil:
	default:
		return nil, fmt.Errorf(
			"unexpected type for Due (items[5]): got %T (%v), expected string [%w]",
			items[5], items[5], ErrValidation)
	}

	switch completed := items[6].(type) {
	case string:
		assignment.Completed = completed
	case nil:
	default:
		return nil, fmt.Errorf(
			"unexpected type for Completed (items[6]): got %T (%v), expected string [%w]",
			items[6], items[6], ErrValidation)
	}

	score, ok := items[7].(float64)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Score (items[7]): got %T (%v), expected number [%w]",
			items[7], items[7], ErrValidation)
	}

	assignment.Score = int(score)

	status, ok := items[8].(string)
	if !ok {
		return nil, fmt.Errorf(
			"unexpected type for Status (items[8]): got %T (%v), expected string [%w]",
			items[8], items[8], ErrValidation)
	}

	assignment.Status = status

	return assignment, nil
}

type assignmentResponseHelper struct {
	Page        int
	Total       int
	Records     int
	Assignments []*model.Assignment
}

func (a *assignmentResponseHelper) UnmarshalJSON(b []byte) error {
	type responseType struct {
		Page        interface{} `json:"page"`
		Total       int         `json:"total"`
		Records     int         `json:"records"`
		Assignments interface{} `json:"rows"`
	}

	r := responseType{} // nolint: exhaustivestruct,varnamelen
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&r); err != nil {
		return fmt.Errorf("%v [%w]", err, ErrMarshal)
	}

	a.Total = r.Total
	a.Records = r.Records

	switch v := r.Page.(type) { // nolint: varnamelen
	case float64:
		a.Page = int(v)
	case int:
		a.Page = v
	case string:
		p, err := strconv.Atoi(v) // nolint: varnamelen
		if err != nil {
			return fmt.Errorf(
				"unexpected value for Page: expected number got %v: %v [%w]",
				v, err, ErrMarshal)
		}

		a.Page = p
	default:
		return fmt.Errorf(
			"unexpected type for Page: got %T (%v), expected int or string [%w]",
			v, v, ErrMarshal)
	}

	switch rows := r.Assignments.(type) {
	case map[string]interface{}:
		if len(rows) > 0 {
			return fmt.Errorf(
				"unexpected type for rows: got map with %d keys, expected []interface{} or empty map [%w]",
				len(rows), ErrMarshal)
		}
	case []interface{}:
		assignments := map[int]*model.Assignment{}

		for _, v := range rows {
			switch cell := v.(type) {
			case map[string]interface{}:
				assignment, err := ToAssignment(cell)
				if err != nil {
					return err
				}

				assignments[assignment.ID] = assignment
			default:
				return fmt.Errorf(
					"unexpected type for cells: got %T, expected map[string]interface{} [%w]",
					cell, ErrMarshal)
			}
		}

		list := make([]*model.Assignment, 0, len(assignments))
		for _, v := range assignments {
			list = append(list, v)
		}

		sort.Slice(list, func(i, j int) bool { return list[i].DueDate().Before(list[j].DueDate()) })

		a.Assignments = list
	default:
		return fmt.Errorf(
			"unexpected type for rows: got %T, expected map[string]interface{} or []interface{} [%w]",
			rows, ErrMarshal)
	}

	return nil
}
