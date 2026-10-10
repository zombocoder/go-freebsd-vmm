//go:build freebsd && amd64

package vmm

import (
	"errors"
	"strconv"
	"strings"
	"syscall"
)

// NoVCPU is the VCPU field of an Error that did not happen on a
// particular vCPU.
const NoVCPU = -1

// Library-level errors, describing a misuse of this API rather than a
// failure reported by the kernel.
//
// Failures that come from the kernel keep their errno instead; see
// Error. There is deliberately no sentinel per errno value, because
// syscall.Errno already compares correctly with errors.Is.
var (
	// ErrClosed is returned by any operation on a VM or vCPU whose
	// context has been closed or destroyed.
	ErrClosed = errors.New("vmm: closed")

	// ErrDestroyed is returned when a VM was destroyed and the
	// operation needed it to still exist.
	ErrDestroyed = errors.New("vmm: destroyed")

	// ErrInvalidArgument is returned for arguments rejected before
	// reaching the kernel.
	ErrInvalidArgument = errors.New("vmm: invalid argument")

	// ErrUnsupported is returned for operations that this FreeBSD
	// release, or this architecture, does not provide.
	ErrUnsupported = errors.New("vmm: unsupported")
)

// Error reports a failed VMM operation, preserving the errno the kernel
// set so that callers can act on it.
//
// Op names the libvmmapi operation that failed, VM the virtual machine
// it was attempted on, and VCPU the vCPU where one is involved, or
// NoVCPU otherwise.
type Error struct {
	Op    string
	VM    string
	VCPU  int
	Errno syscall.Errno
}

// Error renders the failure, for example:
//
//	vmm: vm_open "db01": no such file or directory
//	vmm: vm_get_register "db01" vcpu 2: invalid argument
func (e *Error) Error() string {
	var b strings.Builder

	b.WriteString("vmm: ")
	b.WriteString(e.Op)
	if e.VM != "" {
		b.WriteString(" ")
		b.WriteString(strconv.Quote(e.VM))
	}
	if e.VCPU != NoVCPU {
		b.WriteString(" vcpu ")
		b.WriteString(strconv.Itoa(e.VCPU))
	}
	b.WriteString(": ")
	b.WriteString(e.Errno.Error())

	return b.String()
}

// Unwrap returns the underlying errno, so that callers can write
//
//	if errors.Is(err, syscall.ENOENT) { ... }
func (e *Error) Unwrap() error { return e.Errno }

// newError builds an Error, or nil when errno reports success.
func newError(op, vmName string, vcpu int, errno syscall.Errno) error {
	if errno == 0 {
		return nil
	}
	return &Error{Op: op, VM: vmName, VCPU: vcpu, Errno: errno}
}
