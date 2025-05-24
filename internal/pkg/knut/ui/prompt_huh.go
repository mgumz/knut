// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package ui

import (
	"github.com/charmbracelet/huh"
)

func promptHuh(addrs []string) (string, error) {

	pickedAddr := ""

	form := huh.NewSelect[string]().
		Title("Select the IP to listen on").
		Options(huh.NewOptions(addrs...)...).
		Value(&pickedAddr)

	if err := form.Run(); err != nil {
		return "", err
	}

	return pickedAddr, nil
}
