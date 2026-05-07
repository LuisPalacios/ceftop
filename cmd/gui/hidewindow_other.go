//go:build !windows

package main

import "syscall"

// setHideWindow is a no-op on non-Windows: SysProcAttr.HideWindow does not
// exist on Unix, and there is no console flash to suppress in the first
// place because GUI processes do not own a console there.
func setHideWindow(_ *syscall.SysProcAttr) {}
