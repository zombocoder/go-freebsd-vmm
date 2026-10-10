//go:build freebsd && amd64

package vmm

import (
	"reflect"
	"testing"
)

func TestCPUSetContains(t *testing.T) {
	// CPUs 0, 3 and 64: the last one checks that the second word of
	// the bitmap is read, which is where an off-by-one in the word
	// index would show up.
	s := CPUSet{words: []uint64{1<<0 | 1<<3, 1 << 0}}

	in := []int{0, 3, 64}
	out := []int{1, 2, 4, 63, 65, 127}

	for _, id := range in {
		if !s.Contains(id) {
			t.Errorf("Contains(%d) = false, want true", id)
		}
	}
	for _, id := range out {
		if s.Contains(id) {
			t.Errorf("Contains(%d) = true, want false", id)
		}
	}
}

func TestCPUSetContainsOutOfRange(t *testing.T) {
	s := CPUSet{words: []uint64{1}}

	// Past the end of the bitmap and below zero both answer false
	// rather than panicking.
	for _, id := range []int{-1, -64, 64, 1 << 20} {
		if s.Contains(id) {
			t.Errorf("Contains(%d) = true, want false", id)
		}
	}
}

func TestCPUSetEmpty(t *testing.T) {
	for name, s := range map[string]CPUSet{
		"zero value":  {},
		"empty words": {words: []uint64{0, 0}},
	} {
		if got := s.Len(); got != 0 {
			t.Errorf("%s: Len() = %d, want 0", name, got)
		}
		if got := s.IDs(); len(got) != 0 {
			t.Errorf("%s: IDs() = %v, want empty", name, got)
		}
		if got := s.String(); got != "[]" {
			t.Errorf("%s: String() = %q, want %q", name, got, "[]")
		}
		if s.Contains(0) {
			t.Errorf("%s: Contains(0) = true, want false", name)
		}
	}
}

func TestCPUSetLenAndIDs(t *testing.T) {
	tests := []struct {
		name  string
		words []uint64
		want  []int
	}{
		{name: "single", words: []uint64{1}, want: []int{0}},
		{name: "first four", words: []uint64{0xF}, want: []int{0, 1, 2, 3}},
		{name: "sparse", words: []uint64{1<<5 | 1<<17}, want: []int{5, 17}},
		{name: "top of word", words: []uint64{1 << 63}, want: []int{63}},
		{
			name:  "across words",
			words: []uint64{1 << 63, 1},
			want:  []int{63, 64},
		},
		{
			name:  "full word",
			words: []uint64{^uint64(0)},
			want:  seq(0, 64),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := CPUSet{words: tc.words}
			if got := s.Len(); got != len(tc.want) {
				t.Errorf("Len() = %d, want %d", got, len(tc.want))
			}
			if got := s.IDs(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("IDs() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCPUSetIDsAreSorted matters because callers iterate the result and
// expect vCPU order.
func TestCPUSetIDsAreSorted(t *testing.T) {
	s := CPUSet{words: []uint64{1<<7 | 1<<2, 1<<40 | 1<<3}}

	ids := s.IDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("IDs() = %v, not increasing at index %d", ids, i)
		}
	}
	if want := []int{2, 7, 67, 104}; !reflect.DeepEqual(ids, want) {
		t.Errorf("IDs() = %v, want %v", ids, want)
	}
}

func TestCPUSetString(t *testing.T) {
	s := CPUSet{words: []uint64{1<<0 | 1<<1 | 1<<3}}
	if got, want := s.String(), "0,1,3"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestCPUSetIsAValue checks that copying a CPUSet is safe to do, which
// the documented value semantics promise.
func TestCPUSetIsAValue(t *testing.T) {
	original := CPUSet{words: []uint64{0b1010}}
	copied := original

	if !reflect.DeepEqual(original.IDs(), copied.IDs()) {
		t.Error("copying a CPUSet changed its contents")
	}
}

// seq returns the integers in [from, to).
func seq(from, to int) []int {
	out := make([]int, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}
