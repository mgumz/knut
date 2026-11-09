package handler

import (
	"net/http"
	"os"
)

func FileOrDirHandler(path, uri string) http.Handler {

	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return ServeFileHandler(path)
	}

	handler := DirListHandler(publishedDir(path))
	handler = http.StripPrefix(uri, handler)
	return handler
}
