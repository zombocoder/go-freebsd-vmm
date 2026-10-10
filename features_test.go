//go:build freebsd && amd64

package vmm

import (
	"sort"
	"testing"
)

func TestSupportsKnownFeatures(t *testing.T) {
	// Available on every FreeBSD this package builds on.
	always := []Feature{
		FeatureBulkRegisters,
		FeatureOpenFlags,
		FeatureDeviceMemory,
		FeatureStatistics,
	}
	for _, f := range always {
		if !Supports(f) {
			t.Errorf("Supports(%q) = false, want true", f)
		}
	}
}

func TestSupportsUnknownFeature(t *testing.T) {
	if Supports(Feature("no-such-feature")) {
		t.Error(`Supports("no-such-feature") = true, want false`)
	}
	if Supports(Feature("")) {
		t.Error(`Supports("") = true, want false`)
	}
}

// TestSparseMemoryTracksAPIVersion ties the feature to the reason it
// exists: vm_setup_memory() asserts on anything but MemoryMapAll until
// libvmmapi 3.0.
func TestSparseMemoryTracksAPIVersion(t *testing.T) {
	want := APIVersion().AtLeast(Version{Major: 3})
	if got := Supports(FeatureSparseMemory); got != want {
		t.Errorf("Supports(FeatureSparseMemory) = %v, want %v for libvmmapi %v",
			got, want, APIVersion())
	}
}

func TestFeaturesAreSortedAndAvailable(t *testing.T) {
	got := Features()
	if len(got) == 0 {
		t.Fatal("Features() is empty")
	}

	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] < got[j] }) {
		t.Errorf("Features() = %v, not sorted", got)
	}
	for _, f := range got {
		if !Supports(f) {
			t.Errorf("Features() lists %q but Supports says false", f)
		}
	}
	t.Logf("features: %v", got)
}
