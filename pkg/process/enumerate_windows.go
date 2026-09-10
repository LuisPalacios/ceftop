//go:build windows

package process

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Toolhelp's PROCESSENTRY32 carries cntThreads, so the enrichment pass can
// skip gopsutil's NumThreads() entirely.
const enumerationReportsThreads = true

// enumerateLite walks a single CreateToolhelp32Snapshot and returns PID,
// PPID, exe basename and thread count for every process on the host.
//
// This exists because gopsutil's Windows Ppid() and NumThreads() each
// create their *own* Toolhelp snapshot and scan it linearly for the PID
// (see getFromSnapProcess in process_windows.go). Calling them once per
// process made a tick O(N²): measured at ~3.4 s for 466 processes on a
// Windows 11 desktop, against ~9 ms for one full walk. The exe basename
// comes from the same entry, which also sidesteps gopsutil's Name() —
// that one goes through OpenProcess and silently fails for protected and
// other-user processes, dropping roughly a third of the host from the
// tree and breaking PPID chains that run through them.
func enumerateLite() ([]liteProc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("toolhelp snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return nil, fmt.Errorf("toolhelp first entry: %w", err)
	}

	out := make([]liteProc, 0, 512)
	for {
		out = append(out, liteProc{
			procRef: procRef{
				PID:  int32(pe.ProcessID),
				PPID: int32(pe.ParentProcessID),
				Name: windows.UTF16ToString(pe.ExeFile[:]),
			},
			Threads: int32(pe.Threads),
		})
		if err := windows.Process32Next(snap, &pe); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return out, nil
			}
			return nil, fmt.Errorf("toolhelp next entry: %w", err)
		}
	}
}
