// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

// Package auth holds the logins knut asks for, per window or for all of
// them, as given by -auth and -auth-file.
package auth

import (
	"bufio"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Entry is one login: "[window=]user:password". An entry without a window
// is global.
type Entry struct {
	Window string
	User   string
	secret secret
}

// envPrefix marks a password which is read from the environment.
const envPrefix = "env:"

// bcryptPrefixes are the bcrypt versions CompareHashAndPassword checks.
// "$2y$" is what htpasswd writes.
var bcryptPrefixes = []string{"$2a$", "$2b$", "$2y$"}

// weakPrefixes are the other hashes htpasswd writes. Taking them as plain
// passwords would never let anybody in; taking them as hashes would be
// worse.
var weakPrefixes = []string{"$apr1$", "{SHA}", "$1$", "$5$", "$6$"}

// ParseEntry reads one entry. The errors never quote the password.
func ParseEntry(value string) (Entry, error) {

	var e Entry

	if strings.HasPrefix(value, "/") {
		window, login, ok := strings.Cut(value, "=")
		if !ok {
			return e, fmt.Errorf("window %q: missing \"=\" before the login", value)
		}
		e.Window, value = window, login
	}

	user, password, ok := strings.Cut(value, ":")
	if !ok {
		return e, e.errorf("missing \":\" between name and password")
	}
	if user == "" {
		return e, e.errorf("the name is empty")
	}
	e.User = user

	var err error
	if e.secret, err = parseSecret(password); err != nil {
		return e, e.errorf("%v", err)
	}

	return e, nil
}

func (e Entry) errorf(format string, a ...any) error {
	err := fmt.Sprintf(format, a...)
	switch {
	case e.Window != "" && e.User != "":
		return fmt.Errorf("%s=%s: %s", e.Window, e.User, err)
	case e.Window != "":
		return fmt.Errorf("%s: %s", e.Window, err)
	case e.User != "":
		return fmt.Errorf("%s: %s", e.User, err)
	}
	return errors.New(err)
}

func parseSecret(password string) (secret, error) {

	switch {
	case password == "":
		return nil, errors.New("the password is empty")
	case strings.HasPrefix(password, envPrefix):
		name := password[len(envPrefix):]
		if name == "" {
			return nil, errors.New("\"env:\" needs the name of a variable")
		}
		value := os.Getenv(name)
		if value == "" {
			return nil, fmt.Errorf("$%s is unset or empty", name)
		}
		return plainSecret(value), nil
	case hasPrefix(password, bcryptPrefixes):
		if _, err := bcrypt.Cost([]byte(password)); err != nil {
			return nil, fmt.Errorf("not a valid bcrypt hash: %v", err)
		}
		return bcryptSecret(password), nil
	case hasPrefix(password, weakPrefixes):
		return nil, errors.New("only bcrypt hashes are taken, create one with \"htpasswd -nB <name>\"")
	}

	return plainSecret(password), nil
}

func hasPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// ReadFile reads the entries of an auth file: one entry per line, the same
// as one -auth value. Blank lines and lines starting with "#" are skipped.
// A file other users can access is refused, it holds passwords.
func ReadFile(name string) ([]Entry, error) {

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := checkMode(f, name); err != nil {
		return nil, err
	}

	entries := []Entry{}
	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		e, err := ParseEntry(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, n, err)
		}
		entries = append(entries, e)
	}

	return entries, scanner.Err()
}

func checkMode(f *os.File, name string) error {

	// windows reports permission bits which mean nothing there
	if runtime.GOOS == "windows" {
		return nil
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("%s is open to other users (%04o), it holds passwords: chmod 600 %s", name, perm, name)
	}
	return nil
}

type secret interface {
	matches(password string) bool
}

type plainSecret string

func (s plainSecret) matches(password string) bool {
	return subtle.ConstantTimeCompare([]byte(s), []byte(password)) == 1
}

type bcryptSecret string

func (s bcryptSecret) matches(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(s), []byte(password)) == nil
}
