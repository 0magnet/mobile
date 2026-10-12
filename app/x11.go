// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && !android

package app

/*
Simple on-screen app debugging for X11. Not an officially supported
development target for apps, as screens with mice are very different
than screens with touch panels.
*/

/*
#cgo LDFLAGS: -lEGL -lGLESv2 -lX11

void createWindow(void);
void processEvents(void);
void swapBuffers(void);
int x11Wait(void);
void setSelection(int primary, const char *data, int len);
void requestSelection(int primary);
#include <stdlib.h>
*/
import "C"
import (
	"runtime"
	"time"
	"unsafe"

	"github.com/0magnet/mobile/event/lifecycle"
	"github.com/0magnet/mobile/event/paint"
	"github.com/0magnet/mobile/event/size"
	"github.com/0magnet/mobile/event/touch"
	"github.com/0magnet/mobile/geom"
)

func init() {
	theApp.registerGLViewportFilter()
}

func main(f func(App)) {
	runtime.LockOSThread()

	workAvailable := theApp.worker.WorkAvailable()

	C.createWindow()

	// TODO: send lifecycle events when e.g. the X11 window is iconified or moved off-screen.
	theApp.sendLifecycle(lifecycle.StageFocused)

	// TODO: translate X11 expose events to shiny paint events, instead of
	// sending this synthetic paint event as a hack.
	theApp.eventsIn <- paint.Event{}

	donec := make(chan struct{})
	go func() {
		// close the donec channel in a defer statement
		// so that we could still be able to return even
		// if f panics.
		defer close(donec)

		f(theApp)
	}()

	// The X connection wakes the loop when it has events. Without this they
	// were read only after some other wake (GL work, a publish), so an app
	// that draws only when something changes never heard a key.
	xready, xdrained := make(chan struct{}), make(chan struct{})
	go func() {
		for C.x11Wait() != 0 {
			xready <- struct{}{}
			<-xdrained
		}
	}()
	var draining bool

	// TODO: can we get the actual vsync signal?
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	var tc <-chan time.Time

	for {
		select {
		case <-donec:
			return
		case <-workAvailable:
			theApp.worker.DoWork()
		case <-theApp.publish:
			C.swapBuffers()
			tc = ticker.C
		case s := <-selSet:
			p := C.CString(s.text)
			C.setSelection(boolInt(s.primary), p, C.int(len(s.text)))
			C.free(unsafe.Pointer(p))
		case r := <-selGet:
			if selPending != nil {
				r.reply <- selAnswer{err: errSelectionBusy}
				break
			}
			selPending = r.reply
			C.requestSelection(boolInt(r.primary))
		case <-xready:
			draining = true
		case <-tc:
			tc = nil
			theApp.publishResult <- PublishResult{}
		}
		C.processEvents()
		if draining {
			draining = false
			xdrained <- struct{}{}
		}
	}
}

//export onResize
func onResize(w, h int) {
	// TODO(nigeltao): don't assume 72 DPI. DisplayWidth and DisplayWidthMM
	// is probably the best place to start looking.
	pixelsPerPt := float32(1)
	theApp.eventsIn <- size.Event{
		WidthPx:     w,
		HeightPx:    h,
		WidthPt:     geom.Pt(w),
		HeightPt:    geom.Pt(h),
		PixelsPerPt: pixelsPerPt,
	}
}

func sendTouch(t touch.Type, x, y float32) {
	theApp.eventsIn <- touch.Event{
		X:        x,
		Y:        y,
		Sequence: 0, // TODO: button??
		Type:     t,
	}
}

//export onTouchBegin
func onTouchBegin(x, y float32) { sendTouch(touch.TypeBegin, x, y) }

//export onTouchMove
func onTouchMove(x, y float32) { sendTouch(touch.TypeMove, x, y) }

//export onTouchEnd
func onTouchEnd(x, y float32) { sendTouch(touch.TypeEnd, x, y) }

var stopped bool

//export onStop
func onStop() {
	if stopped {
		return
	}
	stopped = true
	theApp.sendLifecycle(lifecycle.StageDead)
	theApp.eventsIn <- stopPumping{}
}

//export onKey
func onKey(keysym C.ulong, state C.uint, press C.int) {
	theApp.eventsIn <- x11Key(uint32(keysym), uint32(state), press != 0)
}

//export onMouse
func onMouse(x, y float32, button, state C.uint, kind C.int) {
	e, ok := x11Mouse(x, y, uint32(button), uint32(state), int(kind))
	if ok {
		theApp.eventsIn <- e
	}
}

// The clipboard is asked for and set on the main thread, where every Xlib
// call is made: the app's goroutine hands requests over through these.
type selRequest struct {
	primary bool
	text    string
	reply   chan selAnswer
}

type selAnswer struct {
	text string
	err  error
}

var (
	selSet     = make(chan selRequest)
	selGet     = make(chan selRequest)
	selPending chan selAnswer // the read waiting for a SelectionNotify
)

//export onSelection
func onSelection(data *C.char, n C.int, ok C.int) {
	if selPending == nil {
		return
	}
	a := selAnswer{err: errNoSelection}
	if ok != 0 {
		a = selAnswer{text: C.GoStringN(data, n)}
	}
	selPending <- a
	selPending = nil
}

func setClipboard(primary bool, text string) error {
	selSet <- selRequest{primary: primary, text: text}
	return nil
}

func readClipboard(primary bool) (string, error) {
	reply := make(chan selAnswer, 1)
	selGet <- selRequest{primary: primary, reply: reply}
	select {
	case a := <-reply:
		return a.text, a.err
	case <-time.After(2 * time.Second):
		return "", errSelectionTimeout
	}
}

func boolInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
