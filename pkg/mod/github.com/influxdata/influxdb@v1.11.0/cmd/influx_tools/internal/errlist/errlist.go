package errlist

import (
	"bytes"
)

// ErrorList is a simple error aggregator to return multiple errors as one.
type ErrorList struct {
	errs []error
}

// NewErrorList returns an empty *ErrorList
func NewErrorList() *ErrorList {
	return &ErrorList{errs: make([]error, 0)}
}

// Add appends an error to the error list if the it not nil
func (el *ErrorList) Add(err error) {
	if err == nil {
		return
	}
	el.errs = append(el.errs, err)
}

// Err returns whether or not an error list is an error.
func (el *ErrorList) Err() error {
	if len(el.errs) == 0 {
		return nil
	}
	return el
}

// Error satisfies the error interface.
func (el *ErrorList) Error() string {
	var buf bytes.Buffer
	for _, err := range el.errs {
		buf.WriteString(err.Error())
		buf.WriteByte('\n')
	}
	return buf.String()
}
