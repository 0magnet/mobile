// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && !android
// +build linux,!android

#include "_cgo_export.h"
#include <EGL/egl.h>
#include <GLES2/gl2.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <poll.h>

static Atom wm_delete_window;

// The clipboard (CLIPBOARD) and the primary selection (PRIMARY): what this
// window offers while it owns them, index 0 and 1.
static Atom a_clipboard, a_targets, a_utf8, a_prop;
static char *sel_text[2];
static int sel_len[2];

static void answerSelection(XSelectionRequestEvent *r);
static void dropSelection(Atom selection);
static void readSelection(XSelectionEvent *e);

static Window
new_window(Display *x_dpy, EGLDisplay e_dpy, int w, int h, EGLContext *ctx, EGLSurface *surf) {
	static const EGLint attribs[] = {
		EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
		EGL_SURFACE_TYPE, EGL_WINDOW_BIT,
		EGL_BLUE_SIZE, 8,
		EGL_GREEN_SIZE, 8,
		EGL_RED_SIZE, 8,
		EGL_DEPTH_SIZE, 16,
		EGL_CONFIG_CAVEAT, EGL_NONE,
		EGL_NONE
	};
	EGLConfig config;
	EGLint num_configs;
	if (!eglChooseConfig(e_dpy, attribs, &config, 1, &num_configs)) {
		fprintf(stderr, "eglChooseConfig failed\n");
		exit(1);
	}
	EGLint vid;
	if (!eglGetConfigAttrib(e_dpy, config, EGL_NATIVE_VISUAL_ID, &vid)) {
		fprintf(stderr, "eglGetConfigAttrib failed\n");
		exit(1);
	}

	XVisualInfo visTemplate;
	visTemplate.visualid = vid;
	int num_visuals;
	XVisualInfo *visInfo = XGetVisualInfo(x_dpy, VisualIDMask, &visTemplate, &num_visuals);
	if (!visInfo) {
		fprintf(stderr, "XGetVisualInfo failed\n");
		exit(1);
	}

	Window root = RootWindow(x_dpy, DefaultScreen(x_dpy));
	XSetWindowAttributes attr;

	attr.colormap = XCreateColormap(x_dpy, root, visInfo->visual, AllocNone);
	if (!attr.colormap) {
		fprintf(stderr, "XCreateColormap failed\n");
		exit(1);
	}

	attr.event_mask = StructureNotifyMask | ExposureMask |
		ButtonPressMask | ButtonReleaseMask | ButtonMotionMask |
		KeyPressMask | KeyReleaseMask;
	Window win = XCreateWindow(
		x_dpy, root, 0, 0, w, h, 0, visInfo->depth, InputOutput,
		visInfo->visual, CWColormap | CWEventMask, &attr);
	XFree(visInfo);

	XSizeHints sizehints;
	sizehints.width  = w;
	sizehints.height = h;
	sizehints.flags = USSize;
	XSetNormalHints(x_dpy, win, &sizehints);
	XSetStandardProperties(x_dpy, win, "App", "App", None, (char **)NULL, 0, &sizehints);

	// Ask for the keyboard: without the input hint some window managers
	// never give the window focus, and no key event arrives.
	XWMHints *wmhints = XAllocWMHints();
	if (wmhints) {
		wmhints->input = True;
		wmhints->flags = InputHint;
		XSetWMHints(x_dpy, win, wmhints);
		XFree(wmhints);
	}

	static const EGLint ctx_attribs[] = {
		EGL_CONTEXT_CLIENT_VERSION, 2,
		EGL_NONE
	};
	*ctx = eglCreateContext(e_dpy, config, EGL_NO_CONTEXT, ctx_attribs);
	if (!*ctx) {
		fprintf(stderr, "eglCreateContext failed\n");
		exit(1);
	}
	*surf = eglCreateWindowSurface(e_dpy, config, win, NULL);
	if (!*surf) {
		fprintf(stderr, "eglCreateWindowSurface failed\n");
		exit(1);
	}
	return win;
}

Display *x_dpy;
EGLDisplay e_dpy;
EGLContext e_ctx;
EGLSurface e_surf;
Window win;

void
createWindow(void) {
	x_dpy = XOpenDisplay(NULL);
	if (!x_dpy) {
		fprintf(stderr, "XOpenDisplay failed\n");
		exit(1);
	}
	e_dpy = eglGetDisplay(x_dpy);
	if (!e_dpy) {
		fprintf(stderr, "eglGetDisplay failed\n");
		exit(1);
	}
	EGLint e_major, e_minor;
	if (!eglInitialize(e_dpy, &e_major, &e_minor)) {
		fprintf(stderr, "eglInitialize failed\n");
		exit(1);
	}
	eglBindAPI(EGL_OPENGL_ES_API);
	win = new_window(x_dpy, e_dpy, 600, 800, &e_ctx, &e_surf);

	a_clipboard = XInternAtom(x_dpy, "CLIPBOARD", False);
	a_targets = XInternAtom(x_dpy, "TARGETS", False);
	a_utf8 = XInternAtom(x_dpy, "UTF8_STRING", False);
	a_prop = XInternAtom(x_dpy, "GOMOBILE_SELECTION", False);

	wm_delete_window = XInternAtom(x_dpy, "WM_DELETE_WINDOW", True);
	if (wm_delete_window != None) {
		XSetWMProtocols(x_dpy, win, &wm_delete_window, 1);
	}

	XMapWindow(x_dpy, win);
	if (!eglMakeCurrent(e_dpy, e_surf, e_surf, e_ctx)) {
		fprintf(stderr, "eglMakeCurrent failed\n");
		exit(1);
	}

	// Window size and DPI should be initialized before starting app. A window
	// manager configures the window as it maps it; with none, the map is all
	// that comes, and the size is the one asked for. Wait without spinning.
	XEvent ev;
	while (1) {
		XMaskEvent(x_dpy, StructureNotifyMask, &ev);
		if (ev.type == ConfigureNotify) {
			onResize(ev.xconfigure.width, ev.xconfigure.height);
			break;
		}
		if (ev.type == MapNotify) {
			XWindowAttributes wa;
			XGetWindowAttributes(x_dpy, win, &wa);
			onResize(wa.width, wa.height);
			break;
		}
	}
}

void
processEvents(void) {
	while (XPending(x_dpy)) {
		XEvent ev;
		XNextEvent(x_dpy, &ev);
		switch (ev.type) {
		case ButtonPress:
			onMouse((float)ev.xbutton.x, (float)ev.xbutton.y, ev.xbutton.button, ev.xbutton.state, 1);
			if (ev.xbutton.button == Button1) {
				onTouchBegin((float)ev.xbutton.x, (float)ev.xbutton.y);
			}
			break;
		case ButtonRelease:
			onMouse((float)ev.xbutton.x, (float)ev.xbutton.y, ev.xbutton.button, ev.xbutton.state, 2);
			if (ev.xbutton.button == Button1) {
				onTouchEnd((float)ev.xbutton.x, (float)ev.xbutton.y);
			}
			break;
		case KeyPress:
		case KeyRelease: {
			char buf[8];
			KeySym keysym = NoSymbol;
			XLookupString(&ev.xkey, buf, sizeof buf, &keysym, NULL);
			onKey((unsigned long)keysym, ev.xkey.state, ev.type == KeyPress);
			break;
		}
		case MotionNotify:
			onMouse((float)ev.xmotion.x, (float)ev.xmotion.y, 0, ev.xmotion.state, 0);
			if (ev.xmotion.state & Button1Mask) {
				onTouchMove((float)ev.xmotion.x, (float)ev.xmotion.y);
			}
			break;
		case SelectionRequest:
			answerSelection(&ev.xselectionrequest);
			break;
		case SelectionClear:
			dropSelection(ev.xselectionclear.selection);
			break;
		case SelectionNotify:
			readSelection(&ev.xselection);
			break;
		case ConfigureNotify:
			onResize(ev.xconfigure.width, ev.xconfigure.height);
			break;
		case ClientMessage:
			if (wm_delete_window != None && (Atom)ev.xclient.data.l[0] == wm_delete_window) {
				onStop();
				return;
			}
			break;
		}
	}
}

void
swapBuffers(void) {
	if (eglSwapBuffers(e_dpy, e_surf) == EGL_FALSE) {
		fprintf(stderr, "eglSwapBuffer failed\n");
		exit(1);
	}
}

// x11Wait waits until the X connection has something to read, and reports
// whether it has. It touches only the descriptor, never Xlib, so it may run
// on another thread while the main one draws.
int
x11Wait(void) {
	struct pollfd p = { .fd = ConnectionNumber(x_dpy), .events = POLLIN };
	return poll(&p, 1, -1) > 0;
}

static int
selIndex(Atom selection) {
	return selection == XA_PRIMARY ? 1 : 0;
}

// setSelection makes this window the owner of the clipboard (primary 0) or
// the primary selection (primary 1), offering data.
void
setSelection(int primary, const char *data, int len) {
	free(sel_text[primary]);
	sel_text[primary] = malloc(len > 0 ? len : 1);
	memcpy(sel_text[primary], data, len);
	sel_len[primary] = len;
	XSetSelectionOwner(x_dpy, primary ? XA_PRIMARY : a_clipboard, win, CurrentTime);
	XFlush(x_dpy);
}

// requestSelection asks the owner of the clipboard or the primary selection
// for it as UTF-8; the answer comes as a SelectionNotify (readSelection).
void
requestSelection(int primary) {
	XConvertSelection(x_dpy, primary ? XA_PRIMARY : a_clipboard, a_utf8, a_prop, win, CurrentTime);
	XFlush(x_dpy);
}

static void
answerSelection(XSelectionRequestEvent *r) {
	int i = selIndex(r->selection);
	Atom prop = r->property != None ? r->property : r->target; // obsolete clients name none
	XSelectionEvent ne = {0};
	ne.type = SelectionNotify;
	ne.requestor = r->requestor;
	ne.selection = r->selection;
	ne.target = r->target;
	ne.time = r->time;
	ne.property = None;
	if (r->target == a_targets) {
		Atom targets[] = { a_targets, a_utf8, XA_STRING };
		XChangeProperty(x_dpy, r->requestor, prop, XA_ATOM, 32, PropModeReplace,
			(unsigned char *)targets, 3);
		ne.property = prop;
	} else if ((r->target == a_utf8 || r->target == XA_STRING) && sel_text[i]) {
		XChangeProperty(x_dpy, r->requestor, prop, r->target, 8, PropModeReplace,
			(unsigned char *)sel_text[i], sel_len[i]);
		ne.property = prop;
	}
	XSendEvent(x_dpy, r->requestor, False, 0, (XEvent *)&ne);
	XFlush(x_dpy);
}

static void
dropSelection(Atom selection) {
	int i = selIndex(selection);
	free(sel_text[i]);
	sel_text[i] = NULL;
	sel_len[i] = 0;
}

static void
readSelection(XSelectionEvent *e) {
	if (e->property == None) {
		onSelection(NULL, 0, 0);
		return;
	}
	Atom type;
	int format;
	unsigned long n, after;
	unsigned char *data = NULL;
	if (XGetWindowProperty(x_dpy, win, e->property, 0, 1 << 24, True, AnyPropertyType,
			&type, &format, &n, &after, &data) != Success || format != 8) {
		if (data) {
			XFree(data);
		}
		onSelection(NULL, 0, 0); // INCR (very large) transfers are not read
		return;
	}
	onSelection((char *)data, (int)n, 1);
	XFree(data);
}
