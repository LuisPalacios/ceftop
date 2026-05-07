package process

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestKill_RejectsNonPositivePID(t *testing.T) {
	cases := []int32{0, -1, -100}
	for _, pid := range cases {
		got := Kill(pid)
		if got.Killed {
			t.Errorf("Kill(%d).Killed = true, want false", pid)
		}
		if got.Errno != "EINVAL" {
			t.Errorf("Kill(%d).Errno = %q, want %q", pid, got.Errno, "EINVAL")
		}
	}
}

func TestKill_NonExistentPID_ReturnsESRCH(t *testing.T) {
	// PID well above any plausible running PID. Both Linux and Windows
	// have effectively unbounded PID space at the high end, but no real
	// process lives this far up unless something has gone catastrophically
	// wrong on the host.
	const ghost int32 = 2147483646

	got := Kill(ghost)
	if got.Killed {
		t.Fatalf("Kill(%d).Killed = true on a non-existent PID", ghost)
	}
	if got.Errno != "ESRCH" {
		t.Errorf("Kill(%d).Errno = %q, want %q", ghost, got.Errno, "ESRCH")
	}
}

// TestKill_RealSubprocess starts a long-running child process, kills it via
// Kill, then verifies that a second Kill against the same PID returns ESRCH.
// Skips when the host doesn't have the expected sleep tool.
func TestKill_RealSubprocess(t *testing.T) {
	cmd := startSleepHelper(t, 60*time.Second)
	defer reapBestEffort(cmd)

	pid := int32(cmd.Process.Pid)

	got := Kill(pid)
	if !got.Killed {
		t.Fatalf("Kill(%d) failed: %+v", pid, got)
	}

	// Wait for the OS to actually reap the child so the next NewProcess
	// call definitely returns ESRCH instead of racing.
	_ = cmd.Wait()
	// Some platforms keep the PID resolvable for a brief moment after
	// termination; poll a few times.
	for i := 0; i < 20; i++ {
		again := Kill(pid)
		if again.Errno == "ESRCH" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("re-Kill(%d) never returned ESRCH after the process died", pid)
}

// startSleepHelper starts a subprocess that sleeps for d using a portable
// invocation. The Kill test will terminate it before d elapses.
func startSleepHelper(t *testing.T, d time.Duration) *exec.Cmd {
	t.Helper()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// `ping -n` is the most universally available "sleep" on Windows
		// (timeout.exe refuses to run when stdin is redirected, which can
		// trip CI). One ping per second, plus a leading immediate ping.
		seconds := int(d.Seconds())
		if seconds < 2 {
			seconds = 2
		}
		cmd = exec.Command("ping", "-n", itoa(seconds), "127.0.0.1")
	default:
		cmd = exec.Command("sleep", itoa(int(d.Seconds())))
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Skipf("could not start sleep helper: %v", err)
	}
	return cmd
}

func reapBestEffort(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

// itoa avoids strconv to keep the test imports lean.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
