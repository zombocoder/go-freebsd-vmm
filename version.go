//go:build freebsd && amd64

package vmm

import (
	"fmt"

	"github.com/zombocoder/go-freebsd-vmm/internal/vmmapi"
)

// Version is a libvmmapi API version, split into its major and minor
// parts.
type Version struct {
	Major int
	Minor int
}

// String renders the version as "major.minor".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// AtLeast reports whether v is the same as or newer than other.
func (v Version) AtLeast(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	return v.Minor >= other.Minor
}

// APIVersion returns the libvmmapi version this package was built
// against.
//
// FreeBSD 14 reports 2.0 and FreeBSD 15 reports 3.0.
func APIVersion() Version {
	v, err := parseAPIVersion(vmmapi.APIVersionString())
	if err != nil {
		return Version{}
	}
	return v
}

// parseAPIVersion decodes VMMAPI_VERSION from the digits of its source
// spelling, for example "0200" into 2.0.
//
// Decoding the digits rather than the value is deliberate. VMMAPI_VERSION
// is written with a leading zero, so C reads it as octal: 0200 is 128 and
// 0300 is 192, and the arithmetic that the "two digit major followed by
// two digit minor" layout invites gives nonsense. Comparisons between
// versions still work, which is why the encoding survives upstream, but
// extracting the parts does not.
func parseAPIVersion(s string) (Version, error) {
	if len(s) < 3 {
		return Version{}, fmt.Errorf("vmm: malformed VMMAPI_VERSION %q: too short", s)
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return Version{}, fmt.Errorf("vmm: malformed VMMAPI_VERSION %q: not a decimal literal", s)
		}
	}

	split := len(s) - 2
	return Version{
		Major: atoiDigits(s[:split]),
		Minor: atoiDigits(s[split:]),
	}, nil
}

// atoiDigits converts a run of decimal digits that parseAPIVersion has
// already validated.
func atoiDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
