package libris

import (
	"fmt"
	"strings"
)

// Notification implements Martin Fowler's Notification design pattern
// (http://martinfowler.com/articles/replaceThrowWithNotification.html ).
//
// A notification collects errors, so that multiple errors can be accounted for in a single pass instead of failing on
// the first one. The notification can then bundle them all in a single error.
type notification struct {
	// All reported errors.
	Errors []error
}

// Report adds new errors to this notification.
func (n *notification) Report(errs ...error) {
	n.Errors = append(n.Errors, errs...)
}

// Ok returns false if any error was reported.
func (n *notification) Ok() bool {
	return len(n.Errors) == 0
}

// ToError returns a single error instance holding all reported errors, or nil if no error was reported.
func (n *notification) ToError() error {
	if n.Ok() {
		return nil
	}

	return errors(n.Errors)
}

// Errors is an error which bundles and represents the occurrence of multiple errors.
type errors []error

// Error implements the error interface.
func (m errors) Error() string {
	messages := []string{}

	for _, err := range m {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("Multiple errors: %v", strings.Join(messages, "\n\t"))
}
