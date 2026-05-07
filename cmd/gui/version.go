package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetAppVersion returns the application version string for the frontend.
//
//	CI builds:    "v0.2.0 (abc1234)" — version and commit set via ldflags.
//	Local builds: "v0.1.0-3-ga99cf17-dev (a99cf17)" — auto-detected from git.
//	Outside a repo, with no ldflags: "dev-none".
//
// The fallback shells out to `git describe --tags --always` and
// `git rev-parse --short HEAD`. A non-zero exit (shallow clone, no tags, not
// inside a repo) leaves the literal placeholders so the UI can still render
// something rather than an empty string.
func (a *App) GetAppVersion() string {
	v, c := version, commit

	if v == "dev" {
		if tag := gitDescribe(); tag != "" {
			v = tag + "-dev"
		}
	}
	if c == "none" {
		if sha := gitShortSHA(); sha != "" {
			c = sha
		}
	}

	// CI may pass the full 40-char SHA via -ldflags; truncate for display.
	if len(c) > 12 {
		c = c[:7]
	}

	if v == "dev" {
		return fmt.Sprintf("dev-%s", c)
	}
	return fmt.Sprintf("%s (%s)", v, c)
}

func gitDescribe() string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitShortSHA() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
