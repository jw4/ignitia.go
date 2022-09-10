package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/safehtml"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jw4/ignitia.go/pkg/model"
)

// Option represents a function that can modify a session.
type Option func(*Session)

// Assets configures the public assets root folder.
func Assets(path string) Option { return func(s *Session) { s.assets = path } }

// Templates configures the template files root folder.
func Templates(path string) Option { return func(s *Session) { s.templates = path } }

// NewSession returns a Session.
func NewSession(collector Collector, opts ...Option) *Session {
	ses := &Session{
		DebugWriter: os.Stdout,
		assets:      "public",
		templates:   "templates",
		coll:        collector,
	}

	for _, opt := range opts {
		opt(ses)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(ses.assets)))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/index", ses.renderIndex)
	mux.HandleFunc("/report", ses.renderReport)
	ses.mux = mux

	return ses
}

type Collector interface {
	Reset() error
	Data() (model.Data, error)
}

// Session wraps a web session to ignitia.
type Session struct {
	DebugWriter io.Writer

	coll Collector

	data model.Data

	assets    string
	templates string

	mux http.Handler
}

func (s *Session) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		s.mux.ServeHTTP(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Refresh updates the cached data.
func (s *Session) Refresh() error {
	var err error

	if err = s.coll.Reset(); err != nil {
		return err
	}

	s.data, err = s.coll.Data()

	return err
}

// RenderHTML writes the report page out.
func (s *Session) RenderHTML(out io.Writer) error {
	pat := filepath.Join(s.templates, "*.gohtml")
	tmpl, err := template.New("report").Funcs(htmlHelpers).ParseGlob(pat)
	if err != nil {
		return fmt.Errorf("parsing %q: %v", pat, err)
	}

	if err := tmpl.Execute(out, &s.data); err != nil {
		return err
	}

	return nil
}

// RenderJSON writes the report out in JSON.
func (s *Session) RenderJSON(out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(s.data)
}

func (s *Session) renderIndex(writer http.ResponseWriter, _ *http.Request) {
	pat := filepath.Join(s.templates, "*.gohtml")
	tmpl, err := template.New("index").Funcs(htmlHelpers).ParseGlob(pat)
	if err != nil {
		s.renderError(writer, fmt.Errorf("parsing %q: %v", pat, err))
		return
	}

	if err := tmpl.Execute(writer, &s.data); err != nil {
		s.renderError(writer, fmt.Errorf("executing template: %v", err))
		return
	}
}

func (s *Session) renderReport(writer http.ResponseWriter, req *http.Request) {
	if err := s.Refresh(); err != nil {
		s.renderError(writer, err)
		return
	}

	if should(req.FormValue("json")) {
		writer.Header().Set("Content-Type", "application/json")
		if err := s.RenderJSON(writer); err != nil {
			s.renderError(writer, err)
		}

		return
	}

	if err := s.RenderHTML(writer); err != nil {
		s.renderError(writer, err)
		return
	}
}

func (s *Session) renderError(writer http.ResponseWriter, err error) {
	writer.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(s.DebugWriter, "Error serving page: %v\n", err)
}

func should(s string) bool {
	if len(s) == 0 {
		return false
	}

	switch s[0] {
	case '0', 'f', 'F', 'n', 'N', 'x', 'X':
		return false
	default:
		return true
	}
}

func tolower(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		case r >= 'a' && r <= 'z':
			return r
		case r == ' ':
			return '-'
		default:
			return -1
		}
	}, s)
}

var htmlHelpers = template.FuncMap{
	"htmlsafe": func(s string) template.HTML { return template.HTML(safehtml.HTMLEscaped(s).String()) },
	"rawhtml":  func(s string) template.HTML { return template.HTML(s) },
	"tolower":  tolower,
}
