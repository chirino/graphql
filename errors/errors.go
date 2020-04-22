package errors

import (
	"bytes"
	"fmt"
	"github.com/chirino/graphql/text"
	pe "github.com/pkg/errors"
	"io"
	"runtime"
	"strings"
)

type QueryError struct {
	Message       string     `json:"message"`
	Locations     []Location `json:"locations,omitempty"`
	Path          []string   `json:"path,omitempty"`
	Rule          string     `json:"-"`
	ResolverError error      `json:"-"`
	stack         pe.StackTrace
}

type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (l Location) String() string {
	return fmt.Sprintf("(line %d, column %d)", l.Line, l.Column)
}

func (a Location) Before(b Location) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Column < b.Column)
}

func New(msg string) *QueryError {
	return &QueryError{
		Message: msg,
	}
}
func Errorf(format string, a ...interface{}) *QueryError {
	return (&QueryError{
		Message: fmt.Sprintf(format, a...),
	}).WithStack()
}

func (e *QueryError) WithLocations(locations ...Location) *QueryError {
	e.Locations = locations
	return e
}

func (err *QueryError) WithStack() *QueryError {
	err.stack = callers()
	return err
}

func (err *QueryError) Error() string {
	if err == nil {
		return "<nil>"
	}
	str := fmt.Sprintf("graphql: %s", err.Message)
	for _, loc := range err.Locations {
		str += fmt.Sprintf(" %s", loc)
	}

	if len(err.Path) > 0 {
		str += fmt.Sprintf(" (path %s)", strings.Join(err.Path, "/"))
	}
	return str
}

type state struct {
	fmt.State
	buf bytes.Buffer
}

func (s *state) Write(b []byte) (n int, err error) {
	return s.buf.Write(b)
}

func (w *QueryError) Format(s fmt.State, verb rune) {

	type stackTracer interface {
		StackTrace() pe.StackTrace
	}

	switch verb {
	case 'v':
		io.WriteString(s, w.Error())
		if s.Flag('+') {
			stack := w.stack
			if w.Cause() != nil {
				if cause, ok := w.Cause().(stackTracer); ok {
					//fmt.Fprintf(s, "%+v", c)
					stack = cause.StackTrace()
				}
			}
			if stack != nil {
				tempState := &state{State: s}
				stack.Format(tempState, verb)
				stackText := tempState.buf.String()
				io.WriteString(s, "\n"+text.BulletIndent(" stack: ", stackText[1:]))
				io.WriteString(s, "\n")
			}
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, w.Error())
	case 'q':
		fmt.Fprintf(s, "%q", w.Error())
	}
}

func (err *QueryError) Cause() error {
	return err.ResolverError
}

func (err *QueryError) ClearStack() {
	err.stack = nil
}

var _ error = &QueryError{}

func callers() pe.StackTrace {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	f := make([]pe.Frame, n)
	for i := 0; i < n; i++ {
		f[i] = pe.Frame(pcs[i])
	}
	return f
}
