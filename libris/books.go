package libris

import (
	"encoding/csv"
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

// EncodeCSV writes the given books to the given io.Writer. Returns all errors found bundled in a single error, or nil
// if everything went ok.
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
