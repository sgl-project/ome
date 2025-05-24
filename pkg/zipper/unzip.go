package zipper

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func Unzip(zippedFile string, extractingDir string) error {
	r, err := zip.OpenReader(zippedFile)
	if err != nil {
		return err
	}

	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	err = os.MkdirAll(extractingDir, os.ModePerm)
	if err != nil {
		return err
	}

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(extractingDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0777); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
				return err
			}

			_, err := os.Stat(path)
			if err == nil {
				// file exists
				os.Remove(path)
			}

			f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
