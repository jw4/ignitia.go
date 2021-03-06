package ignitia

type Course struct {
	ID          int           `json:"id"`
	Title       string        `json:"title"`
	Assignments []*Assignment `json:"-"`
}

func (c *Course) IncompleteAssignments() int {
	var incomplete int
	for _, a := range c.Assignments {
		if a.IsIncomplete() {
			incomplete++
		}
	}
	return incomplete
}
