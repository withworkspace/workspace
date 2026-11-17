//go:build !darwin && !linux

package store

// networkDisk names the kind of network disk path is on, or "": on this
// system, ws can't tell.
func networkDisk(string) string { return "" }
