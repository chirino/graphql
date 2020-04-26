package qerrors

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/chirino/graphql/text"
	"github.com/pkg/errors"
)

/////////////////////////////////////////////////////////////////////////////
// Section: Error
/////////////////////////////////////////////////////////////////////////////

type Error struct {
	Message       string     `json:"message"`
	Locations     []Location `json:"locations,omitempty"`
	Path          []string   `json:"path,omitempty"`
	Rule          string     `json:"-"`
	ResolverError error      `json:"-"`
	stack         errors.StackTrace
}

// asserts that *Error implements the error interface.
var _ error = &Error{}

func Errorf(format string, a ...interface{}) *Error {
	return New(fmt.Sprintf(format, a...)).WithStack()
}

func New(message string) *Error {
	return (&Error{
		Message: message,
	}).WithStack()
}

func WrapError(err error, message string) *Error {
	return &Error{
		Message:       message,
		ResolverError: err,
	}
}

func (e *Error) WithPath(path ...string) *Error {
	e.Path = path
	return e
}

func (e *Error) WithRule(rule string) *Error {
	e.Rule = rule
	return e
}

func (e *Error) WithLocations(locations ...Location) *Error {
	e.Locations = locations
	return e
}

func (err *Error) WithStack() *Error {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	f := make([]errors.Frame, n)
	for i := 0; i < n; i++ {
		f[i] = errors.Frame(pcs[i])
	}

	err.stack = f
	return err
}

func (err *Error) ClearStack() *Error {
	err.stack = nil
	return err
}

func (err *Error) Error() string {
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

func (w *Error) Format(s fmt.State, verb rune) {
	type stackTracer interface {
		StackTrace() errors.StackTrace
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

func (err *Error) Cause() error {
	return err.ResolverError
}

/////////////////////////////////////////////////////////////////////////////
// Section: Location
/////////////////////////////////////////////////////////////////////////////

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

/////////////////////////////////////////////////////////////////////////////
// Section: ErrorList
/////////////////////////////////////////////////////////////////////////////

type ErrorList []*Error

func AppendErrors(items ErrorList, values ...error) ErrorList {
	for _, err := range values {
		if err == nil {
			continue
		}
		switch err := err.(type) {
		case *Error:
			items = append(items, err)
		case asError:
			for _, e := range err {
				items = AppendErrors(items, e)
			}
		default:
			items = append(items, WrapError(err, err.Error()))
		}
	}
	return items
}

func (items ErrorList) Error() error {
	if len(items) == 0 {
		return nil
	} else {
		return asError(items)
	}
}

/////////////////////////////////////////////////////////////////////////////
// Section: asError
/////////////////////////////////////////////////////////////////////////////

type asError ErrorList

var _ error = &asError{}

func (es asError) Error() string {
	points := make([]string, len(es))
	for i, err := range es {
		points[i] = text.BulletIndent(" * ", err.Error())
	}
	return fmt.Sprintf(
		"%d errors occurred:\n\t%s\n\n",
		len(es), strings.Join(points, "\n\t"))
}
