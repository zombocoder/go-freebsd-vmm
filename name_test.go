//go:build freebsd && amd64

package vmm

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "simple", in: "vm01"},
		{name: "hyphen", in: "govmm-test-1234"},
		{name: "underscore", in: "my_vm"},
		{name: "dot", in: "vm.01"},
		{name: "single char", in: "a"},
		{name: "at limit", in: strings.Repeat("a", MaxNameLen)},

		{name: "empty", in: "", wantErr: true},
		{name: "over limit", in: strings.Repeat("a", MaxNameLen+1), wantErr: true},
		{name: "embedded NUL", in: "vm\x0001", wantErr: true},
		{name: "trailing NUL", in: "vm01\x00", wantErr: true},
		{name: "slash", in: "group/vm01", wantErr: true},
		{name: "traversal", in: "../../dev/null", wantErr: true},
		{name: "dot", in: ".", wantErr: true},
		{name: "dotdot", in: "..", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateName(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("validateName(%q) = nil, want error", tc.in)
				}
				if !errors.Is(err, ErrInvalidArgument) {
					t.Errorf("validateName(%q) = %v, want it to wrap ErrInvalidArgument",
						tc.in, err)
				}
				return
			}
			if err != nil {
				t.Errorf("validateName(%q) failed: %v", tc.in, err)
			}
		})
	}
}

// TestNameValidationRejectsBeforeC checks that a name which cannot
// safely reach C is refused by the public entry points, not just by the
// internal helper.
func TestNameValidationRejectsBeforeC(t *testing.T) {
	bad := []string{"", "vm\x0001", "../escape"}

	for _, name := range bad {
		if err := Create(name); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Create(%q) = %v, want ErrInvalidArgument", name, err)
		}
		if _, err := Open(name); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Open(%q) = %v, want ErrInvalidArgument", name, err)
		}
		if _, err := CreateAndOpen(name); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("CreateAndOpen(%q) = %v, want ErrInvalidArgument", name, err)
		}
	}
}

func TestMaxNameLenMatchesFreeBSD(t *testing.T) {
	// SPECNAMELEN - VM_MAX_PREFIXLEN - VM_MAX_SUFFIXLEN - 1, which is
	// 255 - 10 - 15 - 1 on FreeBSD 13 and later.
	if MaxNameLen < 6 {
		t.Errorf("MaxNameLen = %d, below FreeBSD's VM_MIN_NAMELEN", MaxNameLen)
	}
	t.Logf("MaxNameLen = %d", MaxNameLen)
}
