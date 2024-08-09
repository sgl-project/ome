package casper

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func ExtractNonPrefixObjectName(objectPath string, prefix string) string {
	if !strings.Contains(objectPath, "/") || len(prefix) == 0 {
		return objectPath
	}

	return strings.Replace(objectPath, prefix, "", 1)
}

func CopyByFilePath(sourceFilePath string, targetFilePath string) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %s", sourceFilePath, err.Error())
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy source file %s to target path %s: %s", sourceFilePath, targetFilePath, err.Error())
	}
	return nil
}

func CopyReaderToFilePath(source io.Reader, targetFilePath string) error {
	targetFile, err := os.Create(targetFilePath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %s", targetFilePath, err.Error())
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, source); err != nil {
		return fmt.Errorf("failed to copy source to target path %s: %s", targetFilePath, err.Error())
	}
	return nil
}
