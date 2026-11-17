package store

import "syscall"

// networkDisk names the kind of network disk path is on, or "".
func networkDisk(path string) string {
	var fs syscall.Statfs_t
	if syscall.Statfs(path, &fs) != nil {
		return ""
	}
	switch uint64(fs.Type) {
	case 0x6969:
		return "nfs"
	case 0x517b:
		return "smb"
	case 0xfe534d42:
		return "smb2"
	case 0xff534d42:
		return "cifs"
	case 0x01021997:
		return "9p"
	case 0x65735546:
		return "fuse, which may be one"
	}
	return ""
}
