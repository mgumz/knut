// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// gitShown is how many refs the page of a repository shows, and how many
// commits it shows at a time: the latest ones, the rest is what a clone -
// or the next batch of the log - is for.
const gitShown = 50

const (
	// gitRefParam names the ref a "?zip" of a repository is taken of.
	// without it the zip is of HEAD.
	gitRefParam = "ref"

	// the log of a page goes on at "?from=<commit>&skip=<n>": the commits
	// "from" reaches, the first "skip" left out. "from" is pinned to the
	// commit the page started at, so a commit landing in between does
	// not shift what "skip" leaves out.
	gitFromParam = "from"
	gitSkipParam = "skip"

	// "?log" asks for that part of the log alone, as rows to add to the
	// table on screen, see gitLogBatch.
	gitLogParam = "log"
)

var (
	gitRepoTmpl    = view.Template("git")
	gitLogRowsTmpl = view.Block("git-log-rows")
)

// gitRepo is the page of a repository, or the part of its log htmx adds to
// it.
type gitRepo struct {
	view.Page
	Clone    string // the url to hand "git clone"
	Refs     []gitRef
	Commits  []gitCommit
	More     *gitMore // where the log goes on, nil where it ends
	Newer    string   // the part of the log before this one, see gitLogNewer
	Latest   string   // the start of the log, where that is not Newer
	Older    bool     // the commits are added to a log already on screen
	Summary  string
	LogCount string // how much of the log is on screen
	Watch    string // url a live page polls, empty when it does not
}

// gitRef is a branch or a tag, and the commit it points at.
type gitRef struct {
	Name   string
	Hash   string
	Date   string
	ISO    string
	Head   bool // the branch HEAD is on
	Branch bool
}

// gitCommit is one line of the log.
type gitCommit struct {
	Hash    string
	Full    string // the hash in full, what a page of the log is pinned to
	Date    string
	ISO     string
	Author  string
	Subject string
}

// gitMore is where the log goes on: the next batch as rows for htmx, and
// as a page of its own for a browser without it.
type gitMore struct {
	Batch string
	Page  string
}

// gitRepoPage answers "r" with the page of the repository "gitDir", or the
// part of its log the query asks for.
//
// the first page follows the repository under -live. a later one - the
// log from "?skip" on - is history: it renders once.
func gitRepoPage(w http.ResponseWriter, r *http.Request, gitBinary, gitDir string) {

	from, skip, ok := gitLogWindow(r, gitBinary, gitDir)
	if !ok {
		view.Status(w, http.StatusNotFound)
		return
	}

	if r.URL.Query().Has(gitLogParam) {
		gitLogBatch(w, r, gitBinary, gitDir, from, skip)
		return
	}

	ctx := r.Context()
	read := func() (gitRepo, string, error) { return readGitRepo(ctx, gitBinary, gitDir, from, skip) }
	dirs := func() []string { return gitWatchDirs(ctx, gitBinary, gitDir) }

	var (
		repo  gitRepo
		watch string
		err   error
	)
	if skip == 0 {
		repo, watch, err = view.Poll(r, dirs, read)
	} else {
		repo, _, err = read()
	}
	if err != nil {
		view.Status(w, http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", gitDir, err)
		return
	}

	repo.Page = view.PageFor(r, view.RequestPath(r))
	repo.Clone = view.AbsoluteURL(r)
	repo.Watch = watch
	repo.Newer, repo.Latest = gitLogNewer(from, skip)

	view.WriteFor(w, r, gitRepoTmpl, repo)
}

// gitLogWindow reads which part of the log "r" asks for. no "?skip" is the
// start of it, from HEAD. a "?skip" needs a "?from" naming a commit - it
// is the hash that commit resolves to which is handed on, never the query
// as it came. anything else is not ok.
func gitLogWindow(r *http.Request, gitBinary, gitDir string) (string, int, bool) {

	query := r.URL.Query()
	if !query.Has(gitSkipParam) {
		return "", 0, true
	}

	skip, err := strconv.Atoi(query.Get(gitSkipParam))
	if err != nil || skip < 0 {
		return "", 0, false
	}
	if skip == 0 {
		return "", 0, true
	}

	from, ok := gitCommitOf(r.Context(), gitBinary, gitDir, query.Get(gitFromParam))

	return from, skip, ok
}

// gitCommitOf resolves "ref" to the hash of the commit it names.
func gitCommitOf(ctx context.Context, gitBinary, gitDir, ref string) (string, bool) {

	out, err := runGit(ctx, gitBinary, gitDir,
		"rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	commit := strings.TrimSpace(string(out))

	return commit, err == nil && commit != ""
}

// gitLogBatch answers "r" with the commits from "skip" on as rows, for
// htmx to add below the ones on screen: the row which asked for them is
// replaced, and the last of the new ones asks for the next batch in turn.
//
// the count below the log is sent along out of band. it comes after the
// rows - htmx reads an answer in the context of a table the moment it
// starts with a row, and a row is what it has to start with.
func gitLogBatch(w http.ResponseWriter, r *http.Request, gitBinary, gitDir, from string, skip int) {

	commits, _, err := readGitLog(r.Context(), gitBinary, gitDir, from, skip)
	if err != nil {
		view.Status(w, http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", gitDir, err)
		return
	}

	rows := gitRepo{Commits: commits, Older: true}
	rows.More = gitLogMore(commits, from, skip)
	rows.LogCount = gitLogCount(0, skip+len(commits), rows.More != nil)

	view.Write(w, gitLogRowsTmpl, rows)
}

// readGitRepo reads what the page of a repository shows, and the state of
// it. the refs are read whole for the state and cut down for the page.
func readGitRepo(ctx context.Context, gitBinary, gitDir, from string, skip int) (gitRepo, string, error) {

	refsOut, err := runGit(ctx, gitBinary, gitDir, "for-each-ref", "--sort=-creatordate",
		"--format=%(HEAD)%00%(refname)%00%(refname:short)%00"+
			"%(if)%(*objectname)%(then)%(*objectname:short)%(else)%(objectname:short)%(end)%00"+
			"%(creatordate:iso-strict)",
		"refs/heads", "refs/tags")
	if err != nil {
		return gitRepo{}, "", err
	}

	commits, logOut, err := readGitLog(ctx, gitBinary, gitDir, from, skip)
	if err != nil {
		return gitRepo{}, "", err
	}

	refLines := gitLines(refsOut)
	repo := gitRepo{Refs: parseGitRefs(refLines[:min(len(refLines), gitShown)]), Commits: commits}
	repo.More = gitLogMore(commits, from, skip)
	repo.Summary = gitRefSummary(len(repo.Refs), len(refLines))
	repo.LogCount = gitLogCount(skip, len(commits), repo.More != nil)

	return repo, gitState(refsOut, logOut), nil
}

// readGitLog reads one batch of the log: the commits "from" reaches, or
// HEAD without it, the first "skip" left out. a repository without any
// commit has an empty log, not a broken one.
func readGitLog(ctx context.Context, gitBinary, gitDir, from string, skip int) ([]gitCommit, []byte, error) {

	args := []string{"log", fmt.Sprintf("-n%d", gitShown), fmt.Sprintf("--skip=%d", skip),
		"--format=%h%x00%H%x00%aI%x00%an%x00%s"}
	if from == "" {
		args = append(args, "--ignore-missing", "HEAD")
	} else {
		args = append(args, from)
	}

	out, err := runGit(ctx, gitBinary, gitDir, args...)
	if err != nil {
		return nil, nil, err
	}

	return parseGitLog(gitLines(out)), out, nil
}

// gitLogMore says where the log goes on after "commits", or nil where a
// batch came back short of full - that one was the end of it.
func gitLogMore(commits []gitCommit, from string, skip int) *gitMore {

	if len(commits) < gitShown {
		return nil
	}
	if from == "" {
		from = commits[0].Full
	}

	query := url.Values{
		gitFromParam: {from},
		gitSkipParam: {strconv.Itoa(skip + len(commits))},
	}.Encode()

	return &gitMore{Batch: "?" + gitLogParam + "&" + query, Page: "?" + query}
}

// gitLogNewer says where a later part of the log - the page "?skip" asks
// for - leads back to: the part before it, pinned like it, and the start
// of the log. the start is the first page, which follows HEAD and so shows
// what landed since the pin; one step back from the second part is the
// start already, and then there is no second link to it.
func gitLogNewer(from string, skip int) (string, string) {

	switch {
	case skip == 0:
		return "", ""
	case skip <= gitShown:
		return "./", ""
	}

	query := url.Values{
		gitFromParam: {from},
		gitSkipParam: {strconv.Itoa(skip - gitShown)},
	}.Encode()

	return "?" + query, "./"
}

// parseGitRefs reads the lines "for-each-ref" printed. a tag is shown at
// the commit it tags, not at the tag object an annotated one carries.
func parseGitRefs(lines []string) []gitRef {

	refs := make([]gitRef, 0, len(lines))
	for _, line := range lines {

		fields := strings.Split(line, "\x00")
		if len(fields) != 5 {
			continue
		}

		ref := gitRef{
			Name:   fields[2],
			Hash:   fields[3],
			Head:   fields[0] == "*",
			Branch: strings.HasPrefix(fields[1], "refs/heads/"),
		}
		ref.Date, ref.ISO = gitDate(fields[4])
		refs = append(refs, ref)
	}

	return refs
}

func parseGitLog(lines []string) []gitCommit {

	commits := make([]gitCommit, 0, len(lines))
	for _, line := range lines {

		fields := strings.Split(line, "\x00")
		if len(fields) != 5 {
			continue
		}

		commit := gitCommit{Hash: fields[0], Full: fields[1], Author: fields[3], Subject: fields[4]}
		commit.Date, commit.ISO = gitDate(fields[2])
		commits = append(commits, commit)
	}

	return commits
}

// gitRefSummary counts the refs, and says where the page left out some.
func gitRefSummary(shown, refs int) string {

	if shown < refs {
		return fmt.Sprintf("%d of %s", shown, view.Plural(refs, "ref"))
	}

	return view.Plural(refs, "ref")
}

// gitLogCount says how much of the log is on screen: "count" commits after
// the first "skip". the log is read no further than it is shown, so where
// it goes on it is not known how long it is.
func gitLogCount(skip, count int, more bool) string {

	switch {
	case skip > 0:
		return fmt.Sprintf("commits %d-%d", skip+1, skip+count)
	case more:
		return fmt.Sprintf("the latest %d commits", count)
	}

	return view.Plural(count, "commit")
}

// gitWatchDirs names the folders a commit, a push, a new branch or a tag
// write into: the git dir for HEAD and "packed-refs", and every folder
// below "refs/heads" and "refs/tags" - a watch does not reach into
// subfolders, and "feature/x" is one. a worktree keeps its HEAD in a git
// dir of its own and its refs in the one it shares.
//
// what cannot be found is left out. a live page which misses a move is
// not wrong for long: the next poll reads again whatever the watch said.
func gitWatchDirs(ctx context.Context, gitBinary, gitDir string) []string {

	out, err := runGit(ctx, gitBinary, gitDir, "rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir")
	lines := gitLines(out)
	if err != nil || len(lines) == 0 {
		return []string{gitDir}
	}

	dirs := append([]string{}, lines...)

	common := lines[len(lines)-1]
	dirs = append(dirs, filepath.Join(common, "reftable"))
	for _, refs := range []string{"heads", "tags"} {
		filepath.WalkDir(filepath.Join(common, "refs", refs), func(name string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				dirs = append(dirs, name)
			}
			return nil
		})
	}

	return dirs
}

// gitArchive answers "r" with the tree of a commit of the repository
// "gitDir" as a zip, the way "git archive" packs it: HEAD, or the ref the
// query names. the files inside sit in a folder named after the
// repository, and the download carries that name and the moment it was
// taken, see zipDisposition.
//
// the ref is resolved to a commit before anything is sent: a ref which
// names none is a 404, not a broken download. what reaches "git archive"
// is the hash it resolved to, never the ref as it was asked for.
func gitArchive(w http.ResponseWriter, r *http.Request, gitBinary, gitDir string) {

	ref := r.URL.Query().Get(gitRefParam)
	if ref == "" {
		ref = "HEAD"
	}

	commit, ok := gitCommitOf(r.Context(), gitBinary, gitDir, ref)
	if !ok {
		view.Status(w, http.StatusNotFound)
		return
	}

	base := strings.TrimSuffix(zipBaseName(r), ".git")
	if ref != "HEAD" {
		base += "-" + strings.ReplaceAll(ref, "/", "-")
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", zipDisposition(base))

	cmd := exec.CommandContext(r.Context(), gitBinary, "--git-dir="+gitDir,
		"archive", "--format=zip", "--prefix="+base+"/", commit)
	cmd.Stdout = w

	stderr := &strings.Builder{}
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing zip of %q at %s: %v: %s\n",
			gitDir, commit, err, strings.TrimSpace(stderr.String()))
	}
}
