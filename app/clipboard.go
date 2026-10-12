// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import "errors"

var (
	errNoClipboard      = errors.New("app: no clipboard on this platform yet")
	errNoSelection      = errors.New("app: the clipboard is empty or its owner would not convert it")
	errSelectionBusy    = errors.New("app: a clipboard read is already waiting")
	errSelectionTimeout = errors.New("app: the clipboard's owner did not answer")
)

// SetClipboard puts text on the system clipboard. With primary, it is the
// primary selection instead: on X11 what was last selected, which a middle
// click pastes. Where the platform has no primary selection, primary is the
// clipboard. Call it from the app's goroutine, inside Main.
func SetClipboard(primary bool, text string) error { return setClipboard(primary, text) }

// ReadClipboard returns the text on the system clipboard, or with primary the
// primary selection. It waits for the clipboard's owner, which may be another
// program, for up to two seconds.
func ReadClipboard(primary bool) (string, error) { return readClipboard(primary) }
