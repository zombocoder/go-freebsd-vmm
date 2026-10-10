//go:build freebsd && amd64

package vmm

import "sort"

// Feature names a capability of this library on the system it was built
// for.
//
// A feature answers "can I call this here", not "does libvmmapi export
// some particular symbol". That keeps application code away from FreeBSD
// version arithmetic:
//
//	if vmm.Supports(vmm.FeatureSparseMemory) {
//		// ...
//	}
//
// rather than comparing release numbers.
type Feature string

const (
	// FeatureBulkRegisters reports whether registers can be read and
	// written in batches, one kernel call per batch.
	FeatureBulkRegisters Feature = "bulk-registers"

	// FeatureOpenFlags reports whether OpenWithFlags is available.
	FeatureOpenFlags Feature = "open-flags"

	// FeatureSparseMemory reports whether guest memory can be set up
	// with MemoryMapNone or MemoryMapSparse.
	//
	// False on FreeBSD 14: vm_setup_memory() asserts that the style is
	// MemoryMapAll, so anything else would abort the process rather
	// than return an error.
	FeatureSparseMemory Feature = "sparse-memory"

	// FeatureDeviceMemory reports whether device memory segments can
	// be created and mapped into the guest address space.
	FeatureDeviceMemory Feature = "device-memory"

	// FeatureStatistics reports whether per-vCPU statistics can be
	// read.
	FeatureStatistics Feature = "statistics"

	// FeatureVMRun reports whether a vCPU can be entered and its
	// exits decoded.
	FeatureVMRun Feature = "vm-run"

	// FeatureInterrupts reports whether interrupts and exceptions can
	// be injected.
	FeatureInterrupts Feature = "interrupts"

	// FeaturePassthrough reports whether host PCI devices can be
	// assigned to a guest.
	FeaturePassthrough Feature = "passthrough"

	// FeaturePPTReset reports whether a passthrough device can be
	// reset through libvmmapi.
	//
	// False on the supported releases, which have no entry point for
	// it. VM.ResetPassthrough therefore reports ErrUnsupported.
	FeaturePPTReset Feature = "ppt-reset"

	// FeatureRTC reports whether the guest's real time clock can be
	// read and written.
	FeatureRTC Feature = "rtc"

	// FeatureAddressTranslation reports whether guest linear
	// addresses can be translated and host page tables inspected.
	FeatureAddressTranslation Feature = "address-translation"

	// FeatureGuestCopy reports whether guest memory can be read and
	// written through guest linear addresses.
	FeatureGuestCopy Feature = "guest-copy"

	// FeatureSnapshot reports whether libvmmapi exposes the
	// low-level snapshot primitives.
	//
	// This one is weaker than the others, and the difference matters.
	// Snapshot support is a kernel build option: a kernel built
	// without it has no ioctl behind these calls and refuses them
	// with ENOTTY, which cannot be detected without a VM to try it
	// on. True therefore means the Go and C entry points exist, not
	// that the running kernel will honour them.
	//
	// True also does not mean a restorable VM snapshot can be
	// produced; see VM.SnapshotSave for what these primitives do and
	// do not cover.
	FeatureSnapshot Feature = "snapshot"

	// FeatureCapsicum reports whether a VM's descriptor rights can be
	// limited so the process can enter capability mode.
	FeatureCapsicum Feature = "capsicum"
)

// features holds the resolved answer for every feature this build knows
// about.
var features = map[Feature]bool{
	FeatureBulkRegisters:      true,
	FeatureOpenFlags:          true,
	FeatureDeviceMemory:       true,
	FeatureStatistics:         true,
	FeatureVMRun:              true,
	FeatureInterrupts:         true,
	FeaturePassthrough:        true,
	FeatureRTC:                true,
	FeatureAddressTranslation: true,
	FeatureGuestCopy:          true,
	FeatureSnapshot:           true,
	FeatureCapsicum:           true,

	// Needs a libvmmapi that does not assert on the mapping style.
	FeatureSparseMemory: APIVersion().AtLeast(Version{Major: 3}),

	// No libvmmapi entry point exists for this on any supported
	// release.
	FeaturePPTReset: false,
}

// Supports reports whether a feature is available. An unknown feature
// name reports false.
func Supports(feature Feature) bool {
	return features[feature]
}

// Features returns the available features, sorted by name.
func Features() []Feature {
	out := make([]Feature, 0, len(features))
	for f, ok := range features {
		if ok {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
