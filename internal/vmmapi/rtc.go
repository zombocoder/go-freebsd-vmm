//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import "syscall"

// RTCRead reads one byte of the emulated RTC's CMOS register file.
func (c *Context) RTCRead(offset int) (byte, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}
	coff, ok := toCInt(offset)
	if !ok {
		return 0, syscall.EINVAL
	}

	var value C.uint8_t
	if errno := res(C.govmm_vm_rtc_read(c.ptr, coff, &value)); errno != 0 {
		return 0, errno
	}
	return byte(value), 0
}

// RTCWrite writes one byte of the emulated RTC's CMOS register file.
func (c *Context) RTCWrite(offset int, value byte) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	coff, ok := toCInt(offset)
	if !ok {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_rtc_write(c.ptr, coff, C.uint8_t(value)))
}

// RTCGetTime returns the emulated RTC's time as seconds since the Unix
// epoch.
func (c *Context) RTCGetTime() (int64, syscall.Errno) {
	if !c.valid() {
		return 0, syscall.EINVAL
	}

	var secs C.int64_t
	if errno := res(C.govmm_vm_rtc_gettime(c.ptr, &secs)); errno != 0 {
		return 0, errno
	}
	return int64(secs), 0
}

// RTCSetTime sets the emulated RTC from seconds since the Unix epoch.
func (c *Context) RTCSetTime(secs int64) syscall.Errno {
	if !c.valid() {
		return syscall.EINVAL
	}
	return res(C.govmm_vm_rtc_settime(c.ptr, C.int64_t(secs)))
}
