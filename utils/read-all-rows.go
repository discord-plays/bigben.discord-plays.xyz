package utils

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/gocarina/gocsv"
	"io"
)

func ReadAllRows[T any](r io.Reader, f func(*T)) error {
	csvReader, err := gocsv.NewUnmarshaller(csv.NewReader(r), new(T))
	if err != nil {
		return fmt.Errorf("gocsv.NewUnmarshaller(): %w", err)
	}
	for {
		read, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("csvReader.Read(): %w", err)
		}
		if r, ok := read.(*T); ok {
			f(r)
		}
	}
	return nil
}
