package main

import (
	"net/http"
	"net/http/cgi"
	"os/exec"
)

// https://git-scm.com/docs/git-http-backend

func gitHandler(path, uri string) http.Handler {

	gitBinary, _ := exec.LookPath("git")
	gitHandler := new(cgi.Handler)
	gitHandler.Dir = path
	gitHandler.Root = uri
	gitHandler.Path = gitBinary
	gitHandler.Args = []string{"http-backend"}
	gitHandler.Env = []string{
		"GIT_PROJECT_ROOT=" + path,
		"GIT_HTTP_EXPORT_ALL=1"}

	return gitHandler
}
