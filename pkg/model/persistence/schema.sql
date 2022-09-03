PRAGMA foreign_keys = ON;

--
-- Students
--

CREATE TABLE IF NOT EXISTS student (
  id INTEGER,
  name TEXT,
  CONSTRAINT pk_id PRIMARY KEY (id)
) WITHOUT ROWID, STRICT;


--
-- Courses
--

CREATE TABLE IF NOT EXISTS course (
  id INTEGER,
  title TEXT,
  CONSTRAINT pk_id PRIMARY KEY (id)
) WITHOUT ROWID, STRICT;


CREATE TABLE IF NOT EXISTS student_courses (
  student_id INTEGER,
  course_id INTEGER,
  CONSTRAINT pk_student_course PRIMARY KEY (student_id, course_id),
  CONSTRAINT fk_student FOREIGN KEY (student_id) REFERENCES student (id) ON DELETE CASCADE,
  CONSTRAINT fk_course FOREIGN KEY (course_id) REFERENCES course (id) ON DELETE CASCADE
) WITHOUT ROWID, STRICT;


--
-- Assignments
--

CREATE TABLE IF NOT EXISTS assignment (
  id INTEGER,
  course_id INTEGER,
  unit INTEGER,
  title TEXT,
  assignment_type TEXT,
  CONSTRAINT pk_id PRIMARY KEY (id),
  CONSTRAINT fk_course FOREIGN KEY (course_id) REFERENCES course (id) ON DELETE CASCADE
) WITHOUT ROWID, STRICT;


CREATE TABLE IF NOT EXISTS assignment_history (
  student_id INTEGER,
  assignment_id INTEGER,
  as_of TEXT,
  progress INTEGER,
  due TEXT,
  completed TEXT,
  score INTEGER,
  status TEXT,
  CONSTRAINT pk_student_assignment_history PRIMARY KEY (student_id, assignment_id, as_of),
  CONSTRAINT fk_student FOREIGN KEY (student_id) REFERENCES student (id) ON DELETE CASCADE,
  CONSTRAINT fk_assignment FOREIGN KEY (assignment_id) REFERENCES assignment (id) ON DELETE CASCADE
) WITHOUT ROWID, STRICT;


--
-- Views
--

CREATE VIEW IF NOT EXISTS student_assignments AS
SELECT
  h.student_id,
  a.id,
  a.course_id,
  a.unit,
  a.title,
  a.assignment_type,
  h.as_of,
  h.progress,
  h.due,
  h.completed,
  h.score,
  h.status
FROM
  assignment_history h
  JOIN assignment a ON a.id = h.assignment_id
WHERE
  h.as_of = (
    SELECT MAX(as_of)
    FROM assignment_history h2
    WHERE 
      h.student_id = h2.student_id
      AND h.assignment_id = h2.assignment_id
  )
;

CREATE VIEW IF NOT EXISTS report AS
SELECT
  s.name AS Student,
  c.title AS Course,
  sa.unit AS Unit,
  sa.title AS Assignment,
  sa.assignment_type AS Kind,
  sa.as_of AS 'As Of',
  sa.due AS 'Due Date',
  sa.completed AS 'Completion Date',
  sa.score AS Grade,
  sa.status AS Status,
  julianday(date(sa.as_of)) - julianday(sa.due) AS days
FROM
  student_assignments sa 
  JOIN student s ON s.id = sa.student_id
  JOIN course c ON c.id = sa.course_id
ORDER BY
  Student,
  Course,
  Unit,
  [Completion Date],
  days
;
