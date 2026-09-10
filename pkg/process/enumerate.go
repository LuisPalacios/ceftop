package process

// liteProc is what the platform-specific enumeration pass reports for every
// process on the host: identity, parentage, binary basename, and — where the
// platform hands it over for free — the thread count.
//
// The split between this cheap pass and the per-PID enrichment in
// GopsutilProvider.Snapshot exists because the subtree selection needs
// PID / PPID / name for *every* process, while cmdline, memory and CPU
// times are only needed for the handful of processes that end up in the
// target's subtree. See enumerate_windows.go for why the cheap pass must
// not go through gopsutil's per-process accessors on Windows.
type liteProc struct {
	procRef

	// Threads is the thread count reported by the enumeration pass. Only
	// meaningful when enumerationReportsThreads is true for the platform;
	// otherwise it is 0 and the enrichment pass asks gopsutil instead.
	Threads int32
}
