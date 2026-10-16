// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/auth"
)

// buildGuard collects the logins of -auth-file and -auth. nil: no window
// asks for a login. Whatever is wrong with an entry stops knut: a window
// meant to be protected must not end up open. An entry for a window no
// mapping publishes is not wrong, just unused, and only reported.
func buildGuard(opts *knut.Opts, windows []string) *auth.Guard {

	entries := []auth.Entry{}
	for _, name := range opts.AuthFiles {
		fromFile, err := auth.ReadFile(name)
		if err != nil {
			fatal("-auth-file: %v", err)
		}
		entries = append(entries, fromFile...)
	}
	for _, value := range opts.Auth {
		e, err := auth.ParseEntry(value)
		if err != nil {
			fatal("-auth: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		return nil
	}

	guard, err := auth.NewGuard(entries)
	if err != nil {
		fatal("-auth: %v", err)
	}

	for _, window := range guard.Unknown(windows) {
		fmt.Fprintf(os.Stderr, "info: -auth names %q, no mapping publishes it, the entry is unused\n", window)
	}
	for _, pair := range guard.Nested(windows) {
		fmt.Fprintf(os.Stderr, "warning: %q lies below %q, but does not ask for its login\n", pair[0], pair[1])
	}

	return guard
}
