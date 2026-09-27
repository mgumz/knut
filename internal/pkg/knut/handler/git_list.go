// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

var gitListTmpl = view.Template("git-list")

// errNoRepos is a folder without a repository to list, which is a 404.
var errNoRepos = errors.New("no repository to list")

// gitList is the page of a folder holding repositories.
type gitList struct {
	view.Page
	Repos   []gitListed
	Parent  string // link to the enclosing folder, empty at the top
	Summary string
	Watch   string // url a live page polls, empty when it does not
}

// gitListed is one row of that page: a repository, or a folder which
// holds some.
type gitListed struct {
	Name    string
	URL     string
	Repo    bool
	Branch  string // the branch HEAD is on, empty when it is on none
	Subject string // of the commit HEAD is at
	Date    string
	ISO     string
}

// gitListPage answers "r" with the repositories in "dir": the ones right in
// it, and the folders in it which hold repositories of their own - one
// level down, "org/repo.git". a folder without any is a 404.
func gitListPage(w http.ResponseWriter, r *http.Request, gitBinary, dir string, parent bool) {

	ctx := r.Context()
	read := func() (gitList, string, error) { return readGitList(ctx, gitBinary, dir) }
	dirs := func() []string { return gitListWatchDirs(dir) }

	list, watch, err := view.Poll(r, dirs, read)
	switch {
	case errors.Is(err, errNoRepos):
		view.Status(w, r, http.StatusNotFound)
		return
	case err != nil:
		view.Status(w, r, http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", dir, err)
		return
	}

	list.Page = view.PageFor(r, view.RequestPath(r))
	list.Watch, list.Watched = watch, watch != ""
	if parent {
		list.Parent = "../"
	}

	view.WriteFor(w, r, gitListTmpl, list)
}

// readGitList reads the rows of the page and the state of them.
//
// a repository costs one git per read. the rows are read one after the
// other: a folder of repositories is a handful, not thousands, and a
// listing which fans out would hold that many processes per client.
func readGitList(ctx context.Context, gitBinary, dir string) (gitList, string, error) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return gitList{}, "", err
	}

	list, outs := gitList{}, [][]byte{}
	repos, folders := 0, 0

	for _, entry := range entries {

		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}

		row := gitListed{Name: name + "/", URL: (&url.URL{Path: name}).String() + "/"}
		sub := filepath.Join(dir, name)

		gitDir, isRepo := gitDirOf(sub)
		switch {
		case isRepo:
			out, err := runGit(ctx, gitBinary, gitDir, "log", "-1",
				"--format=%cI%x00%s%x00%D", "--ignore-missing", "HEAD")
			if err != nil {
				// a repository git refuses - broken, or owned by
				// someone else - is listed without its details
				fmt.Fprintf(os.Stderr, "warning: %q: %v\n", sub, err)
			}
			row.Repo = true
			row.Date, row.ISO, row.Subject, row.Branch = parseGitHead(out)
			outs = append(outs, []byte(name), out)
			repos++

		case holdsRepos(sub):
			outs = append(outs, []byte(name))
			folders++

		default:
			continue
		}

		list.Repos = append(list.Repos, row)
	}

	if len(list.Repos) == 0 {
		return gitList{}, "", errNoRepos
	}

	list.Summary = view.Plural(repos, "repo")
	if folders > 0 {
		list.Summary += " ι " + view.Plural(folders, "folder")
	}

	return list, gitState(outs...), nil
}

// parseGitHead reads the one line "log -1" printed about HEAD: when, what,
// and which branch it is on - "%D" says "HEAD -> main, tag: v1.0". a
// repository without a commit printed nothing.
func parseGitHead(out []byte) (date, iso, subject, branch string) {

	fields := strings.Split(string(bytes.TrimRight(out, "\n")), "\x00")
	if len(fields) != 3 {
		return "", "", "", ""
	}

	date, iso = gitDate(fields[0])
	for _, deco := range strings.Split(fields[2], ", ") {
		if name, ok := strings.CutPrefix(deco, "HEAD -> "); ok {
			branch = name
		}
	}

	return date, iso, fields[1], branch
}

// holdsRepos says whether a folder right in "dir" is a repository.
func holdsRepos(dir string) bool {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			if _, isRepo := gitDirOf(filepath.Join(dir, entry.Name())); isRepo {
				return true
			}
		}
	}

	return false
}

// gitListWatchDirs names the folders whose moves change the page: the
// folder itself for repositories which come and go, and for each of them
// the git dir and "refs/heads" for a commit which moves what a row shows.
// a branch in a subfolder is missed until the next poll, see gitWatchDirs.
func gitListWatchDirs(dir string) []string {

	dirs := []string{dir}

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if gitDir, isRepo := gitDirOf(filepath.Join(dir, entry.Name())); isRepo {
			dirs = append(dirs, gitDir, filepath.Join(gitDir, "refs", "heads"))
		}
	}

	return dirs
}
