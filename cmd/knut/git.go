// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
)

// gitHandler serves the given directory via "git http-backend". the advantage
// of using "git http-backend" is that it supports the more clever way of
// offering a git repository (opposite to the dumb http-protocol also possible)
//
// see https://git-scm.com/docs/git-http-backend
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

// cgitHandler will call "cgit" via /uri:cgit://path/to/dir. "cgit" uses
// a configuration file given via the environment variable CGIT_CONFIG. if
// that file is not given, a simple one is created for the user. that created
// file is deleted when **knut** shuts down. it's main purpose is to set the
// scan-path directive to "." which makes cgit scan the directory given via
// the uri. if the user places a "cgitrc" file into the .git folder of a
// scanned git-repo, the "repo.*" options are applied there. eg,
//  knut.git/.git/cgitrc
//                      desc=knut - throws trees out of windows
//
// will make that directory be listed with that description.
func cgitHandler(path, uri string) http.Handler {
	cgitBinary, _ := exec.LookPath("cgit")
	cgitHandler := new(cgi.Handler)
	cgitHandler.Dir = path
	cgitHandler.Root = uri
	cgitHandler.Path = cgitBinary
	cgitHandler.Env = os.Environ()
	return cgitHandler
}
