package utils

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

func WalkFS(filePath string, fn func(string, string, fs.FileInfo) error) error {
	var walk func(string, string, fs.FileInfo) error

	walk = func(virtualPath, filePath string, info fs.FileInfo) error {
		if (info.Mode() & os.ModeSymlink) != 0 {
			targetPath, err := filepath.EvalSymlinks(filePath)
			if err != nil {
				return fmt.Errorf("cannot resolve symlink %q: %w",
					filePath, err)
			}

			virtualPath = filePath
			filePath = targetPath

			info, err = os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("cannot stat %q: %w", filePath, err)
			}
		}

		if info.IsDir() {
			filenames, err := ioutil.ReadDir(filePath)
			if err != nil {
				return fmt.Errorf("cannot list directory %q: %w",
					filePath, err)
			}

			for _, childInfo := range filenames {
				childName := childInfo.Name()
				childVirtualPath := path.Join(virtualPath, childName)
				childFilePath := path.Join(filePath, childName)

				err := walk(childVirtualPath, childFilePath, childInfo)
				if err != nil {
					return err
				}
			}

			return nil
		} else {
			return fn(virtualPath, filePath, info)
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", filePath, err)
	}

	return walk(filePath, filePath, info)
}
