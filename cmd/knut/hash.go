// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/mgumz/knut/internal/pkg/knut/auth"
)

// printHash prints an entry with a bcrypt hash for -auth or an auth file.
func printHash(user string) {

	if err := auth.CheckName(user); err != nil {
		fatal("-hash: %v", err)
	}
	password, err := readPassword(os.Stdin)
	if err != nil {
		fatal("-hash: %v", err)
	}
	entry, err := auth.Hash(user, password)
	if err != nil {
		fatal("-hash: %v", err)
	}
	fmt.Println(entry)
}

// readPassword asks for the password twice, without echo. Not from a
// terminal, the first line of stdin is the password: `echo pw | knut -hash`.
func readPassword(in *os.File) (string, error) {

	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return readLine(in)
	}

	ask := func(prompt string) (string, error) {
		fmt.Fprint(os.Stderr, prompt)
		password, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return string(password), err
	}

	password, err := ask("Password: ")
	if err != nil {
		return "", err
	}
	repeated, err := ask("Repeat:   ")
	if err != nil {
		return "", err
	}
	if password != repeated {
		return "", errors.New("the passwords differ")
	}
	return password, nil
}

func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("reading the password from stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
