package scanner

import "syscall"

func ioctlGrab(fd uintptr, request, value uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall(syscall.SYS_IOCTL, fd, request, value)
}
