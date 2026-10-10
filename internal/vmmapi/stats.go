//go:build freebsd && amd64

package vmmapi

/*
#include "compat.h"
*/
import "C"

import (
	"bytes"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// StatEntry is one statistics counter together with its description.
type StatEntry struct {
	Index       int
	Description string
	Value       uint64
}

// statsMu serialises the statistics path for the whole process.
//
// <vmmapi.h> warns that vm_get_stats() is not MT-safe. On FreeBSD 14 its
// buffer is in fact thread-local, and the genuinely unsafe call is
// vm_get_stat_desc(), which returns a pointer into a process-global
// static struct. One process-wide lock covers both, and the C side
// copies out of that storage before returning, so nothing escapes the
// critical section.
var statsMu sync.Mutex

// statsInitial is the number of counters requested on the first attempt.
// FreeBSD 14 reports fewer than a hundred per vCPU.
const statsInitial = 128

// Stats reads every statistics counter for vcpu, along with the kernel's
// timestamp for the sample.
func (c *Context) Stats(vcpu *VCPU) ([]StatEntry, time.Time, syscall.Errno) {
	if !c.valid() || !vcpu.valid() {
		return nil, time.Time{}, syscall.EINVAL
	}

	statsMu.Lock()
	defer statsMu.Unlock()

	capacity := statsInitial
	for {
		vals := make([]C.uint64_t, capacity)
		descs := make([]byte, capacity*StatDescLen)
		cmax, ok := toCInt(capacity)
		if !ok {
			return nil, time.Time{}, syscall.EINVAL
		}

		var count C.int
		var sec, usec C.int64_t
		r := C.govmm_vm_get_stats(c.ptr, vcpu.ptr, &vals[0],
			(*C.char)(unsafe.Pointer(&descs[0])), cmax, &count,
			&sec, &usec)
		if errno := res(r); errno != 0 {
			if errno == syscall.ENOSPC && int(count) > capacity {
				capacity = int(count)
				continue
			}
			return nil, time.Time{}, errno
		}

		n := int(count)
		if n > capacity {
			// Cannot happen: a short buffer reports ENOSPC.
			return nil, time.Time{}, syscall.EIO
		}

		out := make([]StatEntry, n)
		for i := 0; i < n; i++ {
			raw := descs[i*StatDescLen : (i+1)*StatDescLen]
			if j := bytes.IndexByte(raw, 0); j >= 0 {
				raw = raw[:j]
			}
			out[i] = StatEntry{
				Index:       i,
				Description: string(raw),
				Value:       uint64(vals[i]),
			}
		}
		return out, time.Unix(int64(sec), int64(usec)*1000), 0
	}
}
