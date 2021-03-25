package graphql

import (
	"github.com/chirino/graphql/qerrors"
)

type Error = qerrors.Error
type ErrorList = qerrors.ErrorList

func NewError(message string) *Error {
	return qerrors.New(message)
}

func Errorf(format string, a ...interface{}) *Error {
	return qerrors.New(format, a...)
}
