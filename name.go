//go:build freebsd && amd64

package vmm

import (
	"fmt"
	"strings"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// MaxNameLen is the longest VM name FreeBSD accepts.
//
// The limit comes from devfs: the full path under /dev has to fit in
// SPECNAMELEN once vmm(4) has added its prefix and a bootrom suffix.
// It is 229 on FreeBSD 13 and later.
const MaxNameLen int = vmmapi.MaxNameLen

// validateName checks a VM name before it is handed to C.
//
// The kernel does its own validation and this deliberately does not try
// to reproduce it. What it does cover is the two classes of problem the
// kernel cannot help with: a Go string carrying an embedded NUL would be
// silently truncated on the way into C, and a name containing a path
// separator would make libvmmapi open a device other than the one named,
// since it builds the path as "/dev/vmm/" + name.
func validateName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("vmm: empty VM name: %w", ErrInvalidArgument)

	case len(name) > MaxNameLen:
		return fmt.Errorf("vmm: VM name %q is %d bytes, limit is %d: %w",
			name, len(name), MaxNameLen, ErrInvalidArgument)

	case strings.IndexByte(name, 0) >= 0:
		return fmt.Errorf("vmm: VM name %q contains NUL: %w",
			name, ErrInvalidArgument)

	case strings.ContainsRune(name, '/'):
		return fmt.Errorf("vmm: VM name %q contains a path separator: %w",
			name, ErrInvalidArgument)

	case name == "." || name == "..":
		return fmt.Errorf("vmm: VM name %q is a directory reference: %w",
			name, ErrInvalidArgument)
	}

	return nil
}
