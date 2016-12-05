// Package libris defines the Book type, which represents the user's books, and methods to encode books in JSON or CSV.
package libris

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Book represents information about a volume.
type Book struct {
	Title          string   `json:"title,omitempty"`
	Authors        []string `json:"authors,omitempty"`
	Identifier     string   `json:"identifier,omitempty"`
	IdentifierType string   `json:"identifierType,omitempty"`
	//MyRating int64	 `json:"myRating,omitempty"`
	AverageRating float64 `json:"averageRating,omitempty"`
	Publisher     string  `json:"publisher,omitempty"`
	FileType      string  `json:"fileType,omitempty"`
}

// marshalCSVRow returns the data in b as a CSV row.
func (b *Book) marshalCSVRow() []string {
	return []string{
		fmt.Sprintf("%v", b.Title),
		fmt.Sprintf("%v", strings.Join(b.Authors, ", ")),
		fmt.Sprintf("%v", b.Identifier),
		"", // myRating
		fmt.Sprintf("%.2f", b.AverageRating),
		fmt.Sprintf("%v", b.Publisher),
	}
}

// Books is an alias for a slice of *Book, for methods to hang onto.
type Books []*Book

// marshalCSV returns the data in bs as a slice of CSV rows preceded by a header row.
func (bs Books) marshalCSV() [][]string {
	result := [][]string{}

	result = append(result, []string{"Title", "Author", "ISBN", "My Rating", "Average Rating", "Publisher"})

	for _, b := range bs {
		result = append(result, b.marshalCSVRow())
	}

	return result
}

// EncodeCSV writes the given books to the given io.Writer as CSV. Returns all errors found bundled in a single error,
// or nil if everything went ok.
func (bs Books) EncodeCSV(writer io.Writer) error {
	w := csv.NewWriter(writer)
	records := bs.marshalCSV()

	n := &notification{}

	for _, record := range records {
		if err := w.Write(record); err != nil {
			n.Report(err)
		}
	}

	// Write any buffered data to the underlying writer (standard output).
	w.Flush()

	if err := w.Error(); err != nil {
		n.Report(err)
	}

	return n.ToError()
}

// EncodeJSON writes the given books to the given io.Writer as JSON. Returns all errors found bundled in a single
// error, or nil if everything went ok.
func (bs Books) EncodeJSON(writer io.Writer) error {
	booksJSON, err := json.Marshal(bs)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(writer, "%s", booksJSON)
	if err != nil {
		return err
	}

	return nil
}

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
