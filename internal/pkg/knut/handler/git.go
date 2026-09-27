// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// gitBackendPath matches what "git http-backend" answers below a
// repository - the list of services in its http-backend.c. a git client
// asks for nothing else, so whatever does not match is a reader.
var gitBackendPath = regexp.MustCompile(`(^|/)(` +
	`HEAD|info/refs|objects/info/[^/]+|` +
	`objects/[0-9a-f]{2}/[0-9a-f]{38,62}|` +
	`objects/pack/pack-[0-9a-f]{40,64}\.(pack|idx)|` +
	`git-upload-pack|git-receive-pack)$`)

// GitHandler serves the given directory via "git http-backend". the advantage
// of using "git http-backend" is that it supports the more clever way of
// offering a git repository (opposite to the dumb http-protocol also possible)
//
// a browser gets pages instead: a repository is shown with how to clone
// it, its branches and tags and its latest commits, see gitRepoPage; a
// folder holding repositories lists them, see gitListPage. a page sits at
// the uri a clone is made from - git asks below it for paths of its own,
// see gitBackendPath - so the url in the address bar is the one to clone.
//
// "gitBinary" is the git to run, found by the caller.
//
// see https://git-scm.com/docs/git-http-backend
func GitHandler(gitBinary, path, uri string) http.Handler {

	gitHandler := gitBackend(gitBinary, path, uri)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		rel := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, uri), "/")
		if gitBackendPath.MatchString(rel) {
			gitHandler.ServeHTTP(w, r)
			return
		}

		gitPage(w, r, gitBinary, path, rel)
	})
}

// gitBackend runs "git http-backend" on the repositories below "path".
//
// mapping a folder is trusting what is in it: the backend is told so via
// safe.directory, else it refuses a repository owned by another user than
// the one knut runs as ("dubious ownership") while the pages, which name
// the repository outright, show it anyway. "*" reaches no further than
// GIT_PROJECT_ROOT, the backend enters nothing outside of it. the setting
// travels in GIT_CONFIG_*, the one place besides the system and the global
// config git takes it from.
func gitBackend(gitBinary, path, uri string) *cgi.Handler {
	return &cgi.Handler{
		Dir:  path,
		Root: uri,
		Path: gitBinary,
		Args: []string{"http-backend"},
		Env: []string{
			"GIT_PROJECT_ROOT=" + path,
			"GIT_HTTP_EXPORT_ALL=1",
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=safe.directory",
			"GIT_CONFIG_VALUE_0=*"},
	}
}

// gitPage answers "r" with the page of what sits at "rel" below "root": a
// repository, or a folder of them. anything else is a 404.
func gitPage(w http.ResponseWriter, r *http.Request, gitBinary, root, rel string) {

	dir := filepath.Join(root, filepath.FromSlash(path.Clean("/"+rel)))
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		view.Status(w, r, http.StatusNotFound)
		return
	}

	// a page is a folder, same as a listing
	if !strings.HasSuffix(r.URL.Path, "/") {
		localRedirect(w, r, path.Base(r.URL.Path)+"/")
		return
	}

	gitDir, isRepo := gitDirOf(dir)
	switch {
	case isRepo && r.URL.Query().Has(view.QueryZip):
		gitArchive(w, r, gitBinary, gitDir)
	case isRepo:
		gitRepoPage(w, r, gitBinary, gitDir)
	default:
		gitListPage(w, r, gitBinary, dir, rel != "")
	}
}

// gitDirOf finds the repository at "dir": "dir/.git" for a working tree,
// "dir" itself for a bare one.
//
// it does not walk up the way git does: a folder inside a working tree is
// no repository of its own, and git would answer for the one around it.
func gitDirOf(dir string) (string, bool) {

	// a file is fine too: a worktree or a submodule points at its
	// repository through a ".git" file, and git reads it as such
	if dotGit := filepath.Join(dir, ".git"); exists(dotGit) {
		return dotGit, true
	}

	for _, name := range []string{"HEAD", "objects", "refs"} {
		if !exists(filepath.Join(dir, name)) {
			return "", false
		}
	}

	return dir, true
}

func exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// runGit runs git on the repository "gitDir", which is named outright:
// git looking for one on its own would climb out of the published tree.
// the arguments go to git as they are, no shell in between.
func runGit(ctx context.Context, gitBinary, gitDir string, args ...string) ([]byte, error) {

	out, err := gitCommand(ctx, gitBinary, gitDir, args...).Output()

	if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
		return nil, fmt.Errorf("git %s: %v: %s", args[0], err, bytes.TrimSpace(exitErr.Stderr))
	}

	return out, err
}

// gitCommand is git on the repository "gitDir", reading the config of the
// system and of the repository - what "git http-backend" reads, see
// gitBackend. the global one belongs to whoever runs knut: it is set up
// for working in repositories, and some of it changes what knut parses
// (log.showSignature, i18n.logOutputEncoding).
func gitCommand(ctx context.Context, gitBinary, gitDir string, args ...string) *exec.Cmd {

	cmd := exec.CommandContext(ctx, gitBinary, append([]string{"--git-dir=" + gitDir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull)

	return cmd
}

func gitLines(out []byte) []string {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// gitDate renders a date git spelled in iso 8601 the way a listing does.
func gitDate(iso string) (string, string) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "", ""
	}
	return t.Format("2006-01-02 15:04"), iso
}

// gitState fingerprints what git printed for a page: the page shows
// nothing git did not print, so the same output is the same page.
func gitState(outputs ...[]byte) string {

	hash := fnv.New64a()
	for _, out := range outputs {
		hash.Write(out)
		io.WriteString(hash, "\x00")
	}

	return strconv.FormatUint(hash.Sum64(), 36)
}

// CgitHandler will call "cgit" via /uri:cgit://path/to/dir. "cgit" uses
// a configuration file given via the environment variable CGIT_CONFIG. if
// that file is not given, a simple one is created for the user. that created
// file is deleted when **knut** shuts down. it's main purpose is to set the
// scan-path directive to "." which makes cgit scan the directory given via
// the uri. if the user places a "cgitrc" file into the .git folder of a
// scanned git-repo, the "repo.*" options are applied there. eg,
//
//	knut.git/.git/cgitrc
//	                    desc=knut - throws trees out of windows
//
// will make that directory be listed with that description. "cgitBinary"
// is the cgit to run, found by the caller.
func CgitHandler(cgitBinary, path, uri string) http.Handler {
	cgitHandler := new(cgi.Handler)
	cgitHandler.Dir = path
	cgitHandler.Root = uri
	cgitHandler.Path = cgitBinary
	cgitHandler.Env = os.Environ()
	return cgitHandler
}
