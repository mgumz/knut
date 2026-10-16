// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Hash returns "user:<bcrypt hash>": an entry for -auth or an auth file,
// to be prefixed with "window=" where it is meant for one window.
func Hash(user, password string) (string, error) {

	if err := CheckName(user); err != nil {
		return "", err
	}
	if password == "" {
		return "", errors.New("the password is empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return "", errors.New("bcrypt takes at most 72 bytes of a password")
	}
	if err != nil {
		return "", err
	}

	return user + ":" + string(hash), nil
}

// CheckName reports what keeps user from being the name of an entry.
func CheckName(user string) error {
	switch {
	case user == "":
		return errors.New("the name is empty")
	case strings.Contains(user, ":"):
		return errors.New("a name cannot hold \":\"")
	case strings.HasPrefix(user, "/"):
		return errors.New("a name cannot start with \"/\", that is how a window starts")
	}
	return nil
}
