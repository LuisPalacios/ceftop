package process

import (
	"fmt"

	psprocess "github.com/shirou/gopsutil/v4/process"
)

// KillResult is the structured outcome of a kill attempt. The frontend uses
// Errno to render distinct states (success, permission denied, already gone,
// invalid request) without parsing the human-readable Err string.
type KillResult struct {
	Killed bool   `json:"killed"`
	Err    string `json:"err,omitempty"`
	// Errno is a stable, machine-readable category. One of:
	//   ""        - success (Killed=true)
	//   "EINVAL"  - the supplied PID was not a valid process identifier
	//   "ESRCH"   - the process is not running (already exited or never existed)
	//   "EPERM"   - the kill syscall returned an error (permission, protected
	//               process, OS race) — this catches everything except EINVAL
	//               and ESRCH so the UI can collapse them into a single
	//               "could not kill" branch.
	Errno string `json:"errno,omitempty"`
}

// Kill terminates the process identified by pid. It is cross-platform: on
// Linux/macOS it sends SIGKILL, on Windows it calls TerminateProcess.
//
// The two-step pattern (NewProcess then Kill) lets us classify the common
// failure modes as ESRCH (process not running) vs EPERM (kill itself
// failed). A process that disappears in the gap between the two calls will
// surface as EPERM rather than ESRCH — that race is acceptable; the
// user-facing message is still useful.
func Kill(pid int32) KillResult {
	if pid <= 0 {
		return KillResult{
			Errno: "EINVAL",
			Err:   fmt.Sprintf("invalid PID %d", pid),
		}
	}

	proc, err := psprocess.NewProcess(pid)
	if err != nil {
		return KillResult{
			Errno: "ESRCH",
			Err:   fmt.Sprintf("process %d not found: %v", pid, err),
		}
	}

	if err := proc.Kill(); err != nil {
		return KillResult{
			Errno: "EPERM",
			Err:   fmt.Sprintf("could not kill process %d: %v", pid, err),
		}
	}

	return KillResult{Killed: true}
}
