package server

import (
	"io/fs"
	"os"
)

func stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}
