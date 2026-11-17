package store

import "syscall"

// networkDisk names the kind of network disk path is on, or "".
func networkDisk(path string) string {
	var fs syscall.Statfs_t
	if syscall.Statfs(path, &fs) != nil {
		return ""
	}
	var name []byte
	for _, c := range fs.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	switch kind := string(name); kind {
	case "nfs", "smbfs", "afpfs", "webdav", "ftp", "cifs":
		return kind
	case "osxfuse", "macfuse", "fusefs":
		return kind + ", which may be one"
	}
	return ""
}
