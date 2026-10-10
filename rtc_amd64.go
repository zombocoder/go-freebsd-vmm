//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"time"
)

// RTCRegisters is the size of the emulated RTC's CMOS register file: ten
// time and status registers followed by 114 bytes of NVRAM.
const RTCRegisters = 128

// RTCRead reads one byte of the emulated RTC's CMOS register file.
//
// Offsets 0 through 9 are the time and date registers and 10 through 13
// the status registers; the rest is the NVRAM that a PC firmware uses
// for its own settings.
func (vm *VM) RTCRead(offset int) (byte, error) {
	const op = "vm_rtc_read"

	if err := checkRTCOffset(offset); err != nil {
		return 0, err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return 0, err
	}
	defer release()

	value, errno := ctx.RTCRead(offset)
	if errno != 0 {
		return 0, vm.err(op, errno)
	}
	return value, nil
}

// RTCWrite writes one byte of the emulated RTC's CMOS register file.
func (vm *VM) RTCWrite(offset int, value byte) error {
	const op = "vm_rtc_write"

	if err := checkRTCOffset(offset); err != nil {
		return err
	}

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.RTCWrite(offset, value))
}

// RTCTime returns the time the emulated RTC is keeping.
//
// The kernel tracks it in whole seconds, so the result has no
// sub-second part.
func (vm *VM) RTCTime() (time.Time, error) {
	const op = "vm_rtc_gettime"

	ctx, release, err := vm.context(op)
	if err != nil {
		return time.Time{}, err
	}
	defer release()

	secs, errno := ctx.RTCGetTime()
	if errno != 0 {
		return time.Time{}, vm.err(op, errno)
	}
	return time.Unix(secs, 0), nil
}

// SetRTCTime sets the time the emulated RTC keeps.
//
// Only whole seconds are kept; a sub-second part is discarded. The guest
// reads this through the CMOS registers, so what it makes of the value
// depends on the time zone and format its firmware expects.
func (vm *VM) SetRTCTime(t time.Time) error {
	const op = "vm_rtc_settime"

	ctx, release, err := vm.context(op)
	if err != nil {
		return err
	}
	defer release()

	return vm.err(op, ctx.RTCSetTime(t.Unix()))
}

// checkRTCOffset rejects an offset outside the register file.
func checkRTCOffset(offset int) error {
	if offset < 0 || offset >= RTCRegisters {
		return fmt.Errorf("vmm: rtc offset %d is not 0 to %d: %w",
			offset, RTCRegisters-1, ErrInvalidArgument)
	}
	return nil
}
