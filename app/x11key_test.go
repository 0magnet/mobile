// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && !android

package app

import (
	"testing"

	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/mobile/event/mouse"
)

func TestX11Key(t *testing.T) {
	for _, tc := range []struct {
		keysym, state uint32
		want          key.Event
	}{
		{'a', 0, key.Event{Rune: 'a', Code: key.CodeA, Direction: key.DirPress}},
		{'A', 1, key.Event{Rune: 'A', Code: key.CodeA, Modifiers: key.ModShift, Direction: key.DirPress}},
		{'c', 4, key.Event{Rune: 'c', Code: key.CodeC, Modifiers: key.ModControl, Direction: key.DirPress}},
		{'7', 8, key.Event{Rune: '7', Code: key.Code7, Modifiers: key.ModAlt, Direction: key.DirPress}},
		{0xff0d, 0, key.Event{Rune: -1, Code: key.CodeReturnEnter, Direction: key.DirPress}},
		{0xff52, 0, key.Event{Rune: -1, Code: key.CodeUpArrow, Direction: key.DirPress}},
		{0xffc2, 0, key.Event{Rune: -1, Code: key.CodeF5, Direction: key.DirPress}},
		{0xfe20, 1, key.Event{Rune: -1, Code: key.CodeTab, Modifiers: key.ModShift, Direction: key.DirPress}},
		{0x10003b1, 0, key.Event{Rune: 'α', Direction: key.DirPress}},
		{0xffb3, 0, key.Event{Rune: '3', Code: key.CodeKeypad3, Direction: key.DirPress}},
		{'/', 0, key.Event{Rune: '/', Code: key.CodeSlash, Direction: key.DirPress}},
	} {
		if got := x11Key(tc.keysym, tc.state, true); got != tc.want {
			t.Errorf("x11Key(%#x, %#x) = %+v, want %+v", tc.keysym, tc.state, got, tc.want)
		}
	}
	if got := x11Key('a', 0, false); got.Direction != key.DirRelease {
		t.Errorf("release: %v", got.Direction)
	}
}

func TestX11Mouse(t *testing.T) {
	for _, tc := range []struct {
		button, state uint32
		kind          int
		want          mouse.Event
		ok            bool
	}{
		{1, 0, 1, mouse.Event{X: 3, Y: 4, Button: mouse.ButtonLeft, Direction: mouse.DirPress}, true},
		{3, 1, 2, mouse.Event{X: 3, Y: 4, Button: mouse.ButtonRight, Direction: mouse.DirRelease, Modifiers: key.ModShift}, true},
		{0, 1 << 8, 0, mouse.Event{X: 3, Y: 4}, true}, // a drag with button 1 held
		{4, 0, 1, mouse.Event{X: 3, Y: 4, Button: mouse.ButtonWheelUp, Direction: mouse.DirStep}, true},
		{5, 0, 2, mouse.Event{}, false}, // a wheel release
	} {
		got, ok := x11Mouse(3, 4, tc.button, tc.state, tc.kind)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("x11Mouse(button %d, state %#x, kind %d) = %+v %v, want %+v %v", tc.button, tc.state, tc.kind, got, ok, tc.want, tc.ok)
		}
	}
}
