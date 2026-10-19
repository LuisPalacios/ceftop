//go:build windows

package catalog

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// exeDescription reads the version resource of the image running as pid and
// returns its FileDescription, or ProductName when the description is not
// usable. PROCESS_QUERY_LIMITED_INFORMATION is enough for the image path and
// is granted for most other-user and elevated processes; anything that
// still refuses yields "".
func exeDescription(pid int32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	path := windows.UTF16ToString(buf[:size])

	infoSize, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || infoSize == 0 {
		return ""
	}
	info := make([]byte, infoSize)
	if err := windows.GetFileVersionInfo(path, 0, infoSize, unsafe.Pointer(&info[0])); err != nil {
		return ""
	}

	for _, lang := range translations(info) {
		for _, field := range []string{"FileDescription", "ProductName"} {
			if v := versionString(info, fmt.Sprintf(`\StringFileInfo\%s\%s`, lang, field)); usableDescription(v) {
				return v
			}
		}
	}
	return ""
}

// translations lists the language / code-page blocks the resource declares,
// followed by the US-English fallbacks most binaries use when the
// translation table is missing.
func translations(info []byte) []string {
	var out []string
	var ptr unsafe.Pointer
	var n uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&ptr), &n); err == nil && n >= 4 {
		pairs := unsafe.Slice((*uint16)(ptr), n/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			out = append(out, fmt.Sprintf("%04x%04x", pairs[i], pairs[i+1]))
		}
	}
	return append(out, "040904b0", "040904e4")
}

func versionString(info []byte, subBlock string) string {
	var ptr unsafe.Pointer
	var n uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&info[0]), subBlock, unsafe.Pointer(&ptr), &n); err != nil || n == 0 {
		return ""
	}
	return windows.UTF16ToString(unsafe.Slice((*uint16)(ptr), n))
}
