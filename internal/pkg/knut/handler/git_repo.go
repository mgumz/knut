// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// gitShown is how many commits, and how many refs, the page of a
// repository shows: the latest ones, the rest is what a clone is for.
const gitShown = 50

// gitRefParam names the ref a "?zip" of a repository is taken of. without
// it the zip is of HEAD.
const gitRefParam = "ref"

var gitRepoTmpl = view.Template("git")

// gitRepo is the page of a repository.
type gitRepo struct {
	view.Page
	Clone   string // the url to hand "git clone"
	Refs    []gitRef
	Commits []gitCommit
	Summary string
	Watch   string // url a live page polls, empty when it does not
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
	Date    string
	ISO     string
	Author  string
	Subject string
}

// gitRepoPage answers "r" with the page of the repository "gitDir".
func gitRepoPage(w http.ResponseWriter, r *http.Request, gitBinary, gitDir string) {

	ctx := r.Context()
	read := func() (gitRepo, string, error) { return readGitRepo(ctx, gitBinary, gitDir) }
	dirs := func() []string { return gitWatchDirs(ctx, gitBinary, gitDir) }

	repo, watch, err := view.Poll(r, dirs, read)
	if err != nil {
		view.Status(w, http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", gitDir, err)
		return
	}

	repo.Page = view.PageFor(r, view.RequestPath(r))
	repo.Clone = view.AbsoluteURL(r)
	repo.Watch = watch

	view.WriteFor(w, r, gitRepoTmpl, repo)
}

// readGitRepo reads what the page of a repository shows, and the state of
// it. the refs are read whole for the state and cut down for the page.
func readGitRepo(ctx context.Context, gitBinary, gitDir string) (gitRepo, string, error) {

	refsOut, err := runGit(ctx, gitBinary, gitDir, "for-each-ref", "--sort=-creatordate",
		"--format=%(HEAD)%00%(refname)%00%(refname:short)%00"+
			"%(if)%(*objectname)%(then)%(*objectname:short)%(else)%(objectname:short)%(end)%00"+
			"%(creatordate:iso-strict)",
		"refs/heads", "refs/tags")
	if err != nil {
		return gitRepo{}, "", err
	}

	// a repository without any commit has an empty log, not a broken one
	logOut, err := runGit(ctx, gitBinary, gitDir, "log", fmt.Sprintf("-n%d", gitShown),
		"--format=%h%x00%aI%x00%an%x00%s", "--ignore-missing", "HEAD")
	if err != nil {
		return gitRepo{}, "", err
	}

	refLines := gitLines(refsOut)
	repo := gitRepo{Refs: parseGitRefs(refLines[:min(len(refLines), gitShown)])}
	repo.Commits = parseGitLog(gitLines(logOut))
	repo.Summary = gitRepoSummary(len(repo.Refs), len(refLines), len(repo.Commits))

	return repo, gitState(refsOut, logOut), nil
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
		if len(fields) != 4 {
			continue
		}

		commit := gitCommit{Hash: fields[0], Author: fields[2], Subject: fields[3]}
		commit.Date, commit.ISO = gitDate(fields[1])
		commits = append(commits, commit)
	}

	return commits
}

// gitRepoSummary counts what the page shows, and says where it left out
// some. the log is read no further than it is shown, so a full one does
// not know how long it goes on.
func gitRepoSummary(shown, refs, commits int) string {

	summary := view.Plural(refs, "ref")
	if shown < refs {
		summary = fmt.Sprintf("%d of %s", shown, summary)
	}

	if commits < gitShown {
		return summary + " ι " + view.Plural(commits, "commit")
	}

	return summary + fmt.Sprintf(" ι the latest %d commits", gitShown)
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

	out, err := runGit(r.Context(), gitBinary, gitDir,
		"rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	commit := strings.TrimSpace(string(out))
	if err != nil || commit == "" {
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
