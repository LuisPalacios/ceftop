//go:build windows

package main

import "syscall"

// setHideWindow flips SysProcAttr.HideWindow on Windows so the GUI binary
// does not flash a console window when it spawns a child.
func setHideWindow(attr *syscall.SysProcAttr) {
	attr.HideWindow = true
}
