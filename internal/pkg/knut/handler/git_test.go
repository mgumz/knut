// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// gitIn runs git in "dir" with a fixed identity and clock, and no config
// of the machine the test runs on.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// gitSetup skips a test on a machine without git and keeps the config of
// the one it runs on out of it - the handler inherits the same.
func gitSetup(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git to run")
	}

	for key, value := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "knut",
		"GIT_AUTHOR_EMAIL":    "knut@example.com",
		"GIT_AUTHOR_DATE":     "2026-09-19T15:04:00Z",
		"GIT_COMMITTER_NAME":  "knut",
		"GIT_COMMITTER_EMAIL": "knut@example.com",
		"GIT_COMMITTER_DATE":  "2026-09-19T15:04:00Z",
	} {
		t.Setenv(key, value)
	}
}

// testRepos builds a folder of repositories:
//
//	one/      a working tree: two commits on "main", a branch, a tag,
//	          and a folder of its own
//	bare.git  a bare clone of it
//	empty/    a working tree without a commit
//	plain/    no repository at all
func testRepos(t *testing.T) string {
	t.Helper()
	gitSetup(t)

	root := t.TempDir()
	one := filepath.Join(root, "one")

	if err := os.MkdirAll(filepath.Join(one, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(one, "sub", "a.txt"), []byte("knut"), 0600); err != nil {
		t.Fatal(err)
	}

	gitIn(t, root, "init", "-q", "-b", "main", "one")
	gitIn(t, one, "add", ".")
	gitIn(t, one, "commit", "-q", "-m", "first")
	gitIn(t, one, "commit", "-q", "--allow-empty", "-m", "second <b>bold</b>")
	gitIn(t, one, "branch", "side")
	gitIn(t, one, "tag", "-a", "-m", "release", "v1.0")

	gitIn(t, root, "clone", "-q", "--bare", "one", "bare.git")
	gitIn(t, root, "init", "-q", "-b", "main", "empty")

	if err := os.Mkdir(filepath.Join(root, "plain"), 0700); err != nil {
		t.Fatal(err)
	}

	return root
}

// a browser asking for a repository gets its page
func TestGitSummary(t *testing.T) {

	rec := get(GitHandler(testRepos(t), "/uri/"), "/uri/one/")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		// httptest asks for example.com
		`git clone http://example.com/uri/one/`,
		`main <span class="meta">HEAD</span>`,
		`<td class="name">side</td>`,
		`<td class="name">v1.0</td>`,
		`>first</td>`,
		`second &lt;b&gt;bold&lt;/b&gt;`,
		`2026-09-19 15:04`,
		`3 refs ι 2 commits`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not carry %q", want)
		}
	}
}

// an annotated tag is shown at the commit it tags
func TestGitSummaryTagPointsAtCommit(t *testing.T) {

	root := testRepos(t)
	body := get(GitHandler(root, "/"), "/one/").Body.String()

	head, err := exec.Command("git", "-C", filepath.Join(root, "one"), "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}

	want := `<td class="name">v1.0</td>
      <td class="type"><code>` + strings.TrimSpace(string(head)) + `</code>`
	if !strings.Contains(body, want) {
		t.Errorf("the tag does not point at HEAD %s", head)
	}
}

// a bare repository has a page as well, and a mapping of a single one has
// it at the window itself
func TestGitSummaryBareAndSingle(t *testing.T) {

	root := testRepos(t)

	for _, tc := range []struct {
		handler http.Handler
		target  string
	}{
		{GitHandler(root, "/uri/"), "/uri/bare.git/"},
		{GitHandler(filepath.Join(root, "one"), "/uri/"), "/uri/"},
		{GitHandler(filepath.Join(root, "bare.git"), "/"), "/"},
	} {
		body := get(tc.handler, tc.target).Body.String()
		if !strings.Contains(body, "2 commits") {
			t.Errorf("%q: the page shows no log", tc.target)
		}
	}
}

func TestGitSummaryEmpty(t *testing.T) {

	rec := get(GitHandler(testRepos(t), "/"), "/empty/")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "no commits yet") {
		t.Error("a repository without a commit does not say so")
	}
}

// only a repository, or a folder holding some, has a page: not a folder
// inside a working tree - git would answer for the repository around it -
// and not a path climbing out of the published tree
func TestGitSummaryNotARepository(t *testing.T) {

	root := testRepos(t)
	handler := GitHandler(filepath.Join(root, "plain"), "/")

	for _, target := range []string{"/", "/nope/", "/../one/"} {
		if rec := get(handler, target); rec.Code != http.StatusNotFound {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}

	handler = GitHandler(root, "/")
	for _, target := range []string{"/plain/", "/one/sub/", "/one/sub/a.txt"} {
		if rec := get(handler, target); rec.Code != http.StatusNotFound {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}
}

// the page is a folder, like a listing
func TestGitSummaryRedirectsToFolder(t *testing.T) {

	rec := get(GitHandler(testRepos(t), "/uri/"), "/uri/one?x=1")

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got, want := rec.Header().Get("Location"), "one/?x=1"; got != want {
		t.Errorf("got location %q, want %q", got, want)
	}
}

// the page is read on the machine knut runs on as well, and the url to
// clone from is the one in the address bar there
func TestGitSummaryCloneOnLoopback(t *testing.T) {

	body := getFrom(GitHandler(testRepos(t), "/"), "/one/", "localhost:8080").Body.String()

	if !strings.Contains(body, "git clone http://localhost:8080/one/") {
		t.Error("the page on a loopback host offers no url to clone")
	}
}

// git itself asks below the page, and is answered by git
func TestGitBackendAnswersGit(t *testing.T) {

	rec := get(GitHandler(testRepos(t), "/uri/"), "/uri/one/info/refs?service=git-upload-pack")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/x-git-upload-pack-advertisement"; got != want {
		t.Errorf("got content type %q, want %q", got, want)
	}
}

func TestGitBackendPath(t *testing.T) {

	for rel, want := range map[string]bool{
		"HEAD":                             true,
		"one/HEAD":                         true,
		"one/info/refs":                    true,
		"info/refs":                        true,
		"one/git-upload-pack":              true,
		"one/git-receive-pack":             true,
		"one/objects/info/packs":           true,
		"one/objects/info/http-alternates": true,
		"one/objects/ab/" + strings.Repeat("c", 38):                  true,
		"one/objects/pack/pack-" + strings.Repeat("d", 40) + ".pack": true,
		"one/objects/pack/pack-" + strings.Repeat("d", 40) + ".idx":  true,

		"":                        false,
		"one/":                    false,
		"one":                     false,
		"one/sub/":                false,
		"one/HEADS":               false,
		"one/info/refs/":          false,
		"one/objects/ab/xyz":      false,
		"one/objects/pack/a.pack": false,
	} {
		if got := gitBackendPath.MatchString(rel); got != want {
			t.Errorf("%q: got %v, want %v", rel, got, want)
		}
	}
}

// a folder holding repositories lists them, a folder holding none is left
// out, and a working tree is a row of its own however much it holds
func TestGitList(t *testing.T) {

	root := testRepos(t)
	gitIn(t, filepath.Join(root, "one"), "init", "-q", "nested")

	rec := get(GitHandler(root, "/uri/"), "/uri/")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`<a href="one/">one/</a>`,
		`<a href="bare.git/">bare.git/</a>`,
		`<a href="empty/">empty/</a>`,
		`title="second &lt;b&gt;bold&lt;/b&gt;"`,
		`<td class="type">main</td>`,
		`href="one/?zip"`,
		`3 repos`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the list does not carry %q", want)
		}
	}
	for _, unwanted := range []string{`plain/`, `nested`, `href="empty/?zip"`, `class="dir parent"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the list carries %q", unwanted)
		}
	}
}

// a folder of folders of repositories: "org/repo.git"
func TestGitListFolders(t *testing.T) {

	root := testRepos(t)
	org := filepath.Join(root, "org")
	if err := os.Mkdir(org, 0700); err != nil {
		t.Fatal(err)
	}
	gitIn(t, org, "clone", "-q", "--bare", "../one", "one.git")

	handler := GitHandler(root, "/")

	body := get(handler, "/").Body.String()
	if !strings.Contains(body, `<a href="org/">org/</a></td>
      <td class="type">dir</td>`) {
		t.Error("the folder holding a repository is not listed as a folder")
	}
	if !strings.Contains(body, "3 repos ι 1 folder") {
		t.Error("the summary does not count the folder")
	}

	body = get(handler, "/org/").Body.String()
	for _, want := range []string{`<a href="one.git/">one.git/</a>`, `<a href="../">../</a>`, "1 repo"} {
		if !strings.Contains(body, want) {
			t.Errorf("the list of a folder does not carry %q", want)
		}
	}
}

// zipNamesOf reads a response as a zip and names what is in it.
func zipNamesOf(t *testing.T, body []byte) string {
	t.Helper()
	return strings.Join(zipNames(zipOf(t, body)), " ")
}

// a repository is taken along as the tree of HEAD, in a folder named after
// it - a bare one without its ".git"
func TestGitZip(t *testing.T) {

	handler := GitHandler(testRepos(t), "/uri/")

	for target, want := range map[string]string{
		"/uri/one/?zip":      "one/ one/sub/ one/sub/a.txt",
		"/uri/bare.git/?zip": "bare/ bare/sub/ bare/sub/a.txt",
	} {
		rec := get(handler, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusOK)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != "application/zip" {
			t.Errorf("%q: got content type %q", target, got)
		}
		if got := zipNamesOf(t, rec.Body.Bytes()); got != want {
			t.Errorf("%q: got entries %q, want %q", target, got, want)
		}
	}
}

// a ref names what is taken along, and the download is named after it too
func TestGitZipOfRef(t *testing.T) {

	handler := GitHandler(testRepos(t), "/")

	rec := get(handler, "/one/?zip&ref=v1.0")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "filename=one-v1.0-") {
		t.Errorf("got disposition %q, want one named after the ref", got)
	}
	if got, want := zipNamesOf(t, rec.Body.Bytes()), "one-v1.0/ one-v1.0/sub/ one-v1.0/sub/a.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}

	if body := get(handler, "/one/").Body.String(); !strings.Contains(body, `href="?zip&ref=v1.0"`) {
		t.Error("the row of a ref does not offer its zip")
	}
}

// what names no commit has no zip: a ref which is not there, one which is
// an option to git, a repository without a commit
func TestGitZipMissing(t *testing.T) {

	root := testRepos(t)
	handler := GitHandler(root, "/")
	out := filepath.Join(root, "out.zip")

	for _, target := range []string{
		"/one/?zip&ref=nope",
		"/one/?zip&ref=--output=" + out,
		"/empty/?zip",
	} {
		if rec := get(handler, target); rec.Code != http.StatusNotFound {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}
	if exists(out) {
		t.Error("a ref was taken for an option")
	}
}

var gitLiveURL = regexp.MustCompile(`<div id="(?:repo|repos)" hx-get="([^"]*)"`)

// gitArmedURL is the url a live git page polls, resolved against "page".
func gitArmedURL(t *testing.T, page, body string) string {
	t.Helper()

	match := gitLiveURL.FindStringSubmatch(body)
	if match == nil {
		t.Fatal("the page is not armed for live updates")
	}

	return page + strings.ReplaceAll(match[1], "&amp;", "&")
}

// a live page of a repository is held until a commit lands
func TestGitLiveHoldsUntilCommit(t *testing.T) {

	liveMode(t)
	livePace(t, 5*time.Second)

	root := testRepos(t)
	handler := GitHandler(root, "/")

	for _, page := range []string{"/one/", "/"} {

		poll := gitArmedURL(t, page, get(handler, page).Body.String())

		answered := make(chan string, 1)
		go func() { answered <- getHX(handler, poll).Body.String() }()

		select {
		case <-answered:
			t.Fatalf("%q: the poll was answered before anything happened", page)
		case <-time.After(200 * time.Millisecond):
		}

		subject := "commit seen from " + page
		gitIn(t, filepath.Join(root, "one"), "commit", "-q", "--allow-empty", "-m", subject)

		select {
		case body := <-answered:
			if !strings.Contains(body, subject) {
				t.Errorf("%q: the answer does not carry the new commit", page)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%q: the poll was not answered after the commit", page)
		}
	}
}

// a page which moved between the render and the poll is answered at once
func TestGitLiveAnswersStaleState(t *testing.T) {

	liveMode(t)
	livePace(t, time.Minute)

	handler := GitHandler(testRepos(t), "/")

	for _, target := range []string{"/one/?live=notwhatisthere", "/?live=notwhatisthere"} {
		done := make(chan int, 1)
		go func() { done <- get(handler, target).Code }()

		select {
		case code := <-done:
			if code != http.StatusOK {
				t.Errorf("%q: got status %d", target, code)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q: a stale poll was held instead of being answered", target)
		}
	}
}

// without -live a git page does not poll
func TestGitNotLive(t *testing.T) {

	handler := GitHandler(testRepos(t), "/")

	for _, page := range []string{"/one/", "/"} {
		if body := get(handler, page).Body.String(); gitLiveURL.MatchString(body) {
			t.Errorf("%q: the page polls without -live", page)
		}
	}
}
