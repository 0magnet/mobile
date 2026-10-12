// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux || android

package app

func setClipboard(bool, string) error    { return errNoClipboard }
func readClipboard(bool) (string, error) { return "", errNoClipboard }
