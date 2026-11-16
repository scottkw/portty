package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in about_darwin.m.
void showAboutPanel(void);
*/
import "C"

// showNativeAboutPanel opens the standard AppKit About panel.
func showNativeAboutPanel() { C.showAboutPanel() }
