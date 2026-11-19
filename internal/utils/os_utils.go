package utils

import (
	"fmt"
	"os"
)

func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// OpenRegularFile opens path only if it is a regular file.
// The type is checked before opening, since opening a FIFO without a writer blocks
func OpenRegularFile(path string) (*os.File, os.FileInfo, error) {
	if info, err := os.Stat(path); err != nil {
		return nil, nil, err
	} else if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%+q is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	// the path might be replaced between the checks
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, nil, fmt.Errorf("%+q is not a regular file", path)
	}
	return f, info, nil
}
