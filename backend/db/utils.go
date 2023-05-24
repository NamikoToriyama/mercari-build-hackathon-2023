// Utilities for scoring
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
)

func Initialize(ctx context.Context, db *sql.DB) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	err = putDataSql()
	if err != nil {
		return err
	}

	pattern := filepath.Join(root, "sql", "*.sql")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// TODO(ku-mu): Download data here after publishing data
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	for _, path := range paths {
		log.Printf("Load sql file: %s\n", path)
		f, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to load sql: %s", path))
		}

		if _, err = db.ExecContext(ctx, string(f)); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to exec sql: %s", path))
		}
	}

	return nil
}

func putDataSql() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	dpath := filepath.Join(root, "sql", "10_data.sql")
	_, err = os.Stat(dpath)
	if os.IsNotExist(err) {
		url := "https://storage.googleapis.com/ku-mu-public/hackathon-2023/10_data.sql"
		err = download(dpath, url)
		if err != nil {
			return err
		}
	}

	return nil
}

func download(filePath string, urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed resp.Body.Close: %s", err.Error())
		}
	}()

	out, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Printf("failed out.Close: %s", err.Error())
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
}
