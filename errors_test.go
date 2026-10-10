//go:build freebsd && amd64

package vmm

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "vm only",
			err:  &Error{Op: "vm_open", VM: "db01", VCPU: NoVCPU, Errno: syscall.ENOENT},
			want: `vmm: vm_open "db01": no such file or directory`,
		},
		{
			name: "with vcpu",
			err:  &Error{Op: "vm_get_register", VM: "db01", VCPU: 2, Errno: syscall.EINVAL},
			want: `vmm: vm_get_register "db01" vcpu 2: invalid argument`,
		},
		{
			name: "vcpu zero is shown",
			err:  &Error{Op: "vcpu_reset", VM: "vm01", VCPU: 0, Errno: syscall.EBUSY},
			want: `vmm: vcpu_reset "vm01" vcpu 0: device busy`,
		},
		{
			name: "no vm name",
			err:  &Error{Op: "vm_create", VCPU: NoVCPU, Errno: syscall.EPERM},
			want: `vmm: vm_create: operation not permitted`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}

// TestErrorIsErrno is the contract callers depend on: a kernel failure
// keeps its errno and compares against syscall values.
func TestErrorIsErrno(t *testing.T) {
	err := error(&Error{Op: "vm_open", VM: "db01", VCPU: NoVCPU, Errno: syscall.ENOENT})

	if !errors.Is(err, syscall.ENOENT) {
		t.Error("errors.Is(err, syscall.ENOENT) = false, want true")
	}
	if errors.Is(err, syscall.EBUSY) {
		t.Error("errors.Is(err, syscall.EBUSY) = true, want false")
	}

	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("errors.As(err, *Error) = false, want true")
	}
	if e.Op != "vm_open" || e.VM != "db01" || e.Errno != syscall.ENOENT {
		t.Errorf("unwrapped to %+v, lost a field", e)
	}
}

// TestErrorIsErrnoThroughWrapping checks the errno still compares after
// a caller has added its own context.
func TestErrorIsErrnoThroughWrapping(t *testing.T) {
	inner := error(&Error{Op: "vm_open", VM: "db01", VCPU: NoVCPU, Errno: syscall.EBUSY})
	outer := fmt.Errorf("starting db01: %w", inner)

	if !errors.Is(outer, syscall.EBUSY) {
		t.Error("errors.Is through a wrapped error = false, want true")
	}
}

func TestNewError(t *testing.T) {
	if err := newError("vm_open", "vm01", NoVCPU, 0); err != nil {
		t.Errorf("newError with errno 0 = %v, want nil", err)
	}

	err := newError("vm_open", "vm01", 3, syscall.EACCES)
	if err == nil {
		t.Fatal("newError with a real errno = nil")
	}

	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("newError returned %T, want *Error", err)
	}
	if e.VCPU != 3 {
		t.Errorf("VCPU = %d, want 3", e.VCPU)
	}
}

// TestSentinelsAreDistinct guards against the sentinels being aliases of
// one another, which would make errors.Is match the wrong one.
func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		ErrClosed, ErrDestroyed, ErrInvalidArgument, ErrUnsupported,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true, want false", a, b)
			}
		}
	}
}
