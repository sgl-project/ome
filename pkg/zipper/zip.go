package zipper

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ZipDirectory(directory, outputFilename string) error {
	outFile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	return filepath.Walk(directory, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path to maintain directory structure in the zip archive
		relPath, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Use the relative path as the header's name to preserve folder structure
		header.Name = relPath

		// If it's a directory, add a trailing slash
		if info.IsDir() {
			header.Name += "/"
		}

		zipFile, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done (directories are just metadata in zip files)
		if info.IsDir() {
			return nil
		}

		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		return err
	})
}

func ZipFilesWithPrefixes(directory, outputFilename string, prefixes []string) error {
	outFile, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	return filepath.Walk(directory, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path to maintain directory structure in the zip archive
		relPath, err := filepath.Rel(directory, filePath)
		if err != nil {
			return err
		}

		include := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(relPath, prefix) {
				include = true
				break
			}
		}

		if !include {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Use the relative path as the header's name to preserve folder structure
		header.Name = relPath

		// If it's a directory, add a trailing slash
		if info.IsDir() {
			header.Name += "/"
		}

		zipFile, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a directory, we're done (directories are just metadata in zip files)
		if info.IsDir() {
			return nil
		}

		fsFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		return err
	})
}
