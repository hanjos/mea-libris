package libris

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type Book struct {
	Title          string   `json:"title,omitempty"`
	Authors        []string `json:"authors,omitempty"`
	Identifier     string   `json:"identifier,omitempty"`
	IdentifierType string   `json:"identifierType,omitempty"`
	//MyRating int64	 `json:"myRating,omitempty"`
	AverageRating float64 `json:"averageRating,omitempty"`
	Publisher     string  `json:"publisher,omitempty"`
}

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

type Books []*Book

func (bs Books) marshalCSV() [][]string {
	result := [][]string{}

	result = append(result, []string{"Title", "Author", "ISBN", "My Rating", "Average Rating", "Publisher"})

	for _, b := range bs {
		result = append(result, b.marshalCSVRow())
	}

	return result
}

func (bs Books) EncodeCSV(writer io.Writer) error {
	w := csv.NewWriter(writer)
	records := bs.marshalCSV()

	errors := &Notification{}

	for _, record := range records {
		if err := w.Write(record); err != nil {
			errors.Report(err)
		}
	}

	// Write any buffered data to the underlying writer (standard output).
	w.Flush()

	if err := w.Error(); err != nil {
		errors.Report(err)
	}

	return errors.ToError()
}
