package errors

import (
    "fmt"
    "github.com/chirino/graphql/text"
    "io"
    "strings"
)

type multi []error

func Multi(args ...error) error {
    values := []error{}
    for _, err := range args {
        if err != nil {
            if errs, ok := err.(multi); ok {
                values = append(values, errs...)
            } else {
                values = append(values, err)
            }
        }
    }
    if values == nil {
        return nil
    }
    switch len(values) {
    case 0:
        return nil
    case 1:
        return values[0]
    default:
        return multi(values)
    }
}

func (es multi) Error() string {
    points := make([]string, len(es))
    for i, err := range es {
        points[i] = text.BulletIndent(" * ", err.Error())
    }
    return fmt.Sprintf(
        "%d errors occurred:\n\t%s\n\n",
        len(es), strings.Join(points, "\n\t"))
}

func (es multi) Format(s fmt.State, verb rune) {
    format := "* %"
    if s.Flag('+') {
        format += "+"
    }
    format += string(verb)

    points := make([]string, len(es))
    for i, err := range es {
        points[i] = text.BulletIndent(" * ", fmt.Sprintf(format, err))
    }

    msg := fmt.Sprintf(
        "%d errors occurred:\n\t%s\n\n",
        len(es), strings.Join(points, "\n\t"))

    io.WriteString(s, msg)
}
