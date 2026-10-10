//go:build freebsd && amd64

package vmm

import "testing"

func TestParseAPIVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		// The two spellings that matter: FreeBSD 14 and FreeBSD 15.
		// Both are leading-zero C literals, so the value the
		// compiler sees is octal and the digits are the only
		// correct source for the parts.
		{in: "0200", want: Version{Major: 2, Minor: 0}},
		{in: "0300", want: Version{Major: 3, Minor: 0}},

		{in: "0201", want: Version{Major: 2, Minor: 1}},
		{in: "1005", want: Version{Major: 10, Minor: 5}},
		{in: "100", want: Version{Major: 1, Minor: 0}},
		{in: "000", want: Version{Major: 0, Minor: 0}},

		{in: "", wantErr: true},
		{in: "02", wantErr: true},
		{in: "02x0", wantErr: true},
		{in: "0x200", wantErr: true},
		{in: " 200", wantErr: true},
	}

	for _, tc := range tests {
		got, err := parseAPIVersion(tc.in)
		switch {
		case tc.wantErr && err == nil:
			t.Errorf("parseAPIVersion(%q) = %v, want error", tc.in, got)
		case !tc.wantErr && err != nil:
			t.Errorf("parseAPIVersion(%q) failed: %v", tc.in, err)
		case !tc.wantErr && got != tc.want:
			t.Errorf("parseAPIVersion(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestParseAPIVersionIsNotArithmetic guards the reason the parser decodes
// digits. Reading VMMAPI_VERSION as a number gives 128 for 0200, so any
// division by 100 would report major 1.
func TestParseAPIVersionIsNotArithmetic(t *testing.T) {
	v, err := parseAPIVersion("0200")
	if err != nil {
		t.Fatalf("parseAPIVersion failed: %v", err)
	}
	if v.Major == 0o200/100 {
		t.Errorf("parseAPIVersion decoded the octal value, not the digits: got major %d", v.Major)
	}
	if v.Major != 2 {
		t.Errorf("major = %d, want 2", v.Major)
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		in   Version
		want string
	}{
		{Version{2, 0}, "2.0"},
		{Version{3, 0}, "3.0"},
		{Version{10, 12}, "10.12"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%#v.String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		v, other Version
		want     bool
	}{
		{Version{2, 0}, Version{2, 0}, true},
		{Version{3, 0}, Version{2, 0}, true},
		{Version{2, 0}, Version{3, 0}, false},
		{Version{2, 1}, Version{2, 0}, true},
		{Version{2, 0}, Version{2, 1}, false},
		{Version{10, 0}, Version{9, 99}, true},
	}
	for _, tc := range tests {
		if got := tc.v.AtLeast(tc.other); got != tc.want {
			t.Errorf("%v.AtLeast(%v) = %v, want %v",
				tc.v, tc.other, got, tc.want)
		}
	}
}

// TestAPIVersionIsPlausible checks the version actually compiled in. Any
// FreeBSD this package builds on has the vcpu-handle API, which is
// VMMAPI_VERSION 2.0 or newer.
func TestAPIVersionIsPlausible(t *testing.T) {
	v := APIVersion()
	if !v.AtLeast(Version{Major: 2}) {
		t.Errorf("APIVersion() = %v, want at least 2.0", v)
	}
	t.Logf("built against libvmmapi %v", v)
}
