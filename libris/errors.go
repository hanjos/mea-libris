package libris

import (
	"fmt"
	"strings"
)

type Notification struct {
	Errors []error
}

func (n *Notification) Report(errs ...error) {
	n.Errors = append(n.Errors, errs...)
}

func (n *Notification) HasErrors() bool {
	return len(n.Errors) != 0
}

func (n *Notification) ToError() error {
	if !n.HasErrors() {
		return nil
	}

	return errors(n.Errors)
}

type errors []error

func (m errors) Error() string {
	messages := []string{}

	for _, err := range m {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("Multiple errors:%v", strings.Join(messages, "\n\t"))
}
