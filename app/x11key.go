// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && !android

package app

import (
	"github.com/0magnet/mobile/event/key"
	"github.com/0magnet/mobile/event/mouse"
)

// X11 keysyms (X11/keysymdef.h) of the keys that have a Code of their own.
var x11Codes = map[uint32]key.Code{
	0xff0d: key.CodeReturnEnter, 0xff1b: key.CodeEscape, 0xff08: key.CodeDeleteBackspace,
	0xff09: key.CodeTab, 0xfe20: key.CodeTab, // ISO_Left_Tab is Shift+Tab
	0xff13: key.CodePause, 0xff63: key.CodeInsert, 0xff50: key.CodeHome, 0xff55: key.CodePageUp,
	0xffff: key.CodeDeleteForward, 0xff57: key.CodeEnd, 0xff56: key.CodePageDown,
	0xff53: key.CodeRightArrow, 0xff51: key.CodeLeftArrow, 0xff54: key.CodeDownArrow, 0xff52: key.CodeUpArrow,
	0xffe5: key.CodeCapsLock, 0xff7f: key.CodeKeypadNumLock, 0xff8d: key.CodeKeypadEnter,
	0xffaf: key.CodeKeypadSlash, 0xffaa: key.CodeKeypadAsterisk, 0xffad: key.CodeKeypadHyphenMinus,
	0xffab: key.CodeKeypadPlusSign, 0xffae: key.CodeKeypadFullStop, 0xffbd: key.CodeKeypadEqualSign,
	0xffe3: key.CodeLeftControl, 0xffe1: key.CodeLeftShift, 0xffe9: key.CodeLeftAlt, 0xffeb: key.CodeLeftGUI,
	0xffe4: key.CodeRightControl, 0xffe2: key.CodeRightShift, 0xffea: key.CodeRightAlt, 0xffec: key.CodeRightGUI,
	0xff6a: key.CodeHelp,
	' ':    key.CodeSpacebar, '-': key.CodeHyphenMinus, '=': key.CodeEqualSign, '[': key.CodeLeftSquareBracket,
	']': key.CodeRightSquareBracket, '\\': key.CodeBackslash, ';': key.CodeSemicolon, '\'': key.CodeApostrophe,
	'`': key.CodeGraveAccent, ',': key.CodeComma, '.': key.CodeFullStop, '/': key.CodeSlash,
}

// x11Key is the key event for an X11 key event: keysym as XLookupString
// reports it (so with Shift applied: 'A', not 'a'), and the modifier state.
func x11Key(keysym, state uint32, press bool) key.Event {
	e := key.Event{Rune: -1, Direction: key.DirRelease}
	if press {
		e.Direction = key.DirPress
	}
	e.Modifiers = x11Mods(state)

	switch {
	case keysym >= 0x20 && keysym <= 0x7e, keysym >= 0xa0 && keysym <= 0xff:
		e.Rune = rune(keysym) // Latin-1 keysyms are their own code points
	case keysym&0xff000000 == 0x01000000:
		e.Rune = rune(keysym & 0x00ffffff) // the Unicode keysyms
	case keysym >= 0xffb0 && keysym <= 0xffb9:
		e.Rune = rune('0' + keysym - 0xffb0) // the keypad's digits, with Num Lock
		e.Code = key.CodeKeypad0
		if keysym > 0xffb0 {
			e.Code = key.CodeKeypad1 + key.Code(keysym-0xffb1)
		}
		return e
	case keysym >= 0xffbe && keysym <= 0xffc9:
		e.Code = key.CodeF1 + key.Code(keysym-0xffbe)
		return e
	}

	switch r := e.Rune; {
	case r >= 'a' && r <= 'z':
		e.Code = key.CodeA + key.Code(r-'a')
	case r >= 'A' && r <= 'Z':
		e.Code = key.CodeA + key.Code(r-'A')
	case r == '0':
		e.Code = key.Code0
	case r >= '1' && r <= '9':
		e.Code = key.Code1 + key.Code(r-'1')
	default:
		e.Code = x11Codes[keysym]
	}
	return e
}

// x11Mods is the modifiers held in an X11 event's state.
func x11Mods(state uint32) key.Modifiers {
	const shift, ctrl, alt, super = 1 << 0, 1 << 2, 1 << 3, 1 << 6 // ShiftMask, ControlMask, Mod1Mask, Mod4Mask
	var m key.Modifiers
	if state&shift != 0 {
		m |= key.ModShift
	}
	if state&ctrl != 0 {
		m |= key.ModControl
	}
	if state&alt != 0 {
		m |= key.ModAlt
	}
	if state&super != 0 {
		m |= key.ModMeta
	}
	return m
}

// x11Mouse is the mouse event for an X11 button event (kind 1 press, 2
// release) or motion (kind 0). The wheel is buttons 4 to 7, one DirStep per
// press; its releases mean nothing and report false.
func x11Mouse(x, y float32, button, state uint32, kind int) (mouse.Event, bool) {
	e := mouse.Event{X: x, Y: y, Modifiers: x11Mods(state)}
	switch kind {
	case 1:
		e.Direction = mouse.DirPress
	case 2:
		e.Direction = mouse.DirRelease
	}
	switch button {
	case 0:
	case 1:
		e.Button = mouse.ButtonLeft
	case 2:
		e.Button = mouse.ButtonMiddle
	case 3:
		e.Button = mouse.ButtonRight
	case 4, 5, 6, 7:
		if kind != 1 {
			return e, false
		}
		e.Button = [...]mouse.Button{mouse.ButtonWheelUp, mouse.ButtonWheelDown, mouse.ButtonWheelLeft, mouse.ButtonWheelRight}[button-4]
		e.Direction = mouse.DirStep
	default:
		return e, false
	}
	return e, true
}
