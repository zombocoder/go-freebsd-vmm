//go:build freebsd && amd64

package vmm

import "testing"

// Benchmarks for the pure-Go layer. These run anywhere; the ones that
// need a kernel live in tests/vcpu.

func BenchmarkCPUSetIDs(b *testing.B) {
	// A busy 64-CPU host.
	s := CPUSet{words: []uint64{^uint64(0)}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.IDs()
	}
}

func BenchmarkCPUSetContains(b *testing.B) {
	s := CPUSet{words: []uint64{0x5555555555555555, 0xaaaaaaaaaaaaaaaa}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.Contains(i % 128)
	}
}

func BenchmarkParseAPIVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := parseAPIVersion("0200"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if err := validateName("govmm-test-12345-abcdef01"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkErrorString(b *testing.B) {
	err := &Error{Op: "vm_get_register", VM: "db01", VCPU: 2, Errno: 22}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}
