package qerrors

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/chirino/graphql/text"
	uperrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/pkg/errors"
)

/////////////////////////////////////////////////////////////////////////////
// Section: Error
/////////////////////////////////////////////////////////////////////////////

type Error struct {
	*uperrors.QueryError
	PathStr      []string   `json:"path,omitempty"`
	stack     errors.StackTrace
}

// asserts that *Error implements the error interface.
var _ error = &Error{}

func New(message string, a ...interface{}) *Error {
	return (&Error{
		QueryError: uperrors.Errorf(message, a),
	}).WithStack()
}

func WrapError(err error, message string) *Error {
	return &Error{
		QueryError: &uperrors.QueryError {
			Message: message,
			ResolverError:   err,
		},
	}
}

func (e *Error) WithPath(path ...string) *Error {
	e.PathStr = path
	return e
}

func (e *Error) WithCause(err error) *Error {
	e.ResolverError = err
	return e
}

func (e *Error) WithRule(rule string) *Error {
	e.Rule = rule
	return e
}

func (e *Error) WithLocations(locations ...uperrors.Location) *Error {
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
	str := err.QueryError.Error()
	if len(err.PathStr) > 0 {
		str += fmt.Sprintf(" (path %s)", strings.Join(err.PathStr, "/"))
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

type location uperrors.Location

func (l location) String() string {
	return fmt.Sprintf("(line %d, column %d)", l.Line, l.Column)
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
