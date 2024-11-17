package domain

import (
	"fmt"
	"io"
	"os"
)

func copyFileIfNotExist(srcPath, dstPath AnimePath) error {
	// Check if the destination file exists
	if _, err := os.Stat(string(dstPath)); err == nil {
		fmt.Printf("File %s already exists, skipping copy.\n", dstPath)
		return nil // File exists, no need to copy
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check destination file: %v", err)
	}

	// Open the source file
	srcFile, err := os.Open(string(srcPath))
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := os.Create(string(dstPath))
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dstFile.Close()

	// Copy the contents from src to dst
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy content: %v", err)
	}

	fmt.Printf("File copied from %s to %s successfully.\n", srcPath, dstPath)
	return nil
}
