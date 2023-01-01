package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func DownloadData(cacheDir, downloadUrl string) error {
	err := os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("os.MkdirAll(): %w", err)
	}

	get, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	if get.StatusCode != 200 {
		return fmt.Errorf("invalid status code %d: %s", get.StatusCode, func() string {
			b, _ := io.ReadAll(get.Body)
			return string(b)
		}())
	}

	buf := new(bytes.Buffer)
	teeReader := io.TeeReader(get.Body, buf)

	fj := filepath.Join(cacheDir, "final.tar.gz")
	create, err := os.Create(fj)
	if err != nil {
		return fmt.Errorf("os.Create(): %w", err)
	}
	_, err = io.Copy(create, teeReader)
	if err != nil {
		return fmt.Errorf("io.Copy(): %w", err)
	}

	return ProcessData(cacheDir, buf)
}
