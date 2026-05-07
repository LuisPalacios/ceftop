package process

import "regexp"

// Role is the Chromium architectural role of a process. Values are returned
// verbatim from the --type=<role> flag, with one synthetic value
// (RoleMain) reserved for the parent / browser process which has no flag.
//
// The set below tracks the prototype's $RoleColors table — adding a new role
// here is the supported way to teach the GUI about a flag we have not seen
// before; the role string flows straight through to the frontend's role-color
// theme.
type Role string

const (
	RoleMain        Role = "Main / Browser"
	RoleRenderer    Role = "renderer"
	RoleGPUProcess  Role = "gpu-process"
	RoleUtility     Role = "utility"
	RoleCrashpad    Role = "crashpad-handler"
	RoleWatcher     Role = "watcher"
	RolePlugin      Role = "plugin"
	RolePPAPI       Role = "ppapi"
	RolePPAPIBroker Role = "ppapi-broker"
	RoleExtension   Role = "extension"
	RoleZygote      Role = "zygote"
)

// typeFlagRe captures the role argument from a Chromium-style --type=<role>
// command-line flag. The character class matches the prototype's regex
// exactly: lowercase / uppercase letters, digits, hyphen.
var typeFlagRe = regexp.MustCompile(`--type=([a-zA-Z0-9\-]+)`)

// ExtractRole returns the Chromium role implied by a process command line.
// An empty cmdline or a cmdline without a --type= flag is the parent /
// browser process; everything else carries its role verbatim.
func ExtractRole(cmdline string) Role {
	if cmdline == "" {
		return RoleMain
	}
	m := typeFlagRe.FindStringSubmatch(cmdline)
	if len(m) < 2 {
		return RoleMain
	}
	return Role(m[1])
}
