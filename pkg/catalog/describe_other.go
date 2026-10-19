//go:build !windows

package catalog

// exeDescription has no platform source outside Windows yet; FriendlyName
// falls through to the heuristic. A macOS implementation could read the
// enclosing .app bundle's CFBundleDisplayName.
func exeDescription(_ int32) string {
	return ""
}
