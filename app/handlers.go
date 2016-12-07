package app

import (
	"fmt"
	"net/http"
)

// Handler runs the given function and sends the data in *Error, if any, to http.Error.
type Handler func(w http.ResponseWriter, r *http.Request) *Error

// ServeHTTP implements the http.Handler interface.
func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		//logErr.Println(err)
		http.Error(w, err.Message, err.Status)
	}
}

// ERRORS

// Error represents an error in processing, to be converted into the appropriate HTTP status code and message.
type Error struct {
	Message string
	Status  int
}

// Error implements the error interface.
func (err *Error) Error() string {
	return fmt.Sprintf("[%d %s] %s", err.Status, http.StatusText(err.Status), err.Message)
}

// Wrap builds an app.Error from an error and status code. If err is nil or an *app.Error, it will returned unmodified.
func Wrap(err error, status int) *Error {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*Error); ok {
		return appErr
	}

	appErr := &Error{err.Error(), status}

	return appErr
}
