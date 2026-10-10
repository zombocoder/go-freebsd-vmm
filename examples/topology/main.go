//go:build freebsd && amd64

// Command topology sets a VM's CPU topology.
//
//	topology vm01 1 4 1      # 1 socket, 4 cores, 1 thread
//
// The kernel only accepts a topology before the VM's vCPUs have been
// activated, and reports EBUSY afterwards.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("topology: ")

	if len(os.Args) != 5 {
		log.Fatalf("usage: %s <vm-name> <sockets> <cores> <threads>", os.Args[0])
	}

	sockets := mustAtoi(os.Args[2], "sockets")
	cores := mustAtoi(os.Args[3], "cores")
	threads := mustAtoi(os.Args[4], "threads")

	vm, err := vmm.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	before, err := vm.Topology()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("before: %v\n", before)

	want := vmm.Topology{
		Sockets: sockets,
		Cores:   cores,
		Threads: threads,
		MaxCPUs: sockets * cores * threads,
	}
	if err := vm.SetTopology(want); err != nil {
		log.Fatal(err)
	}

	after, err := vm.Topology()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("after:  %v\n", after)
}

func mustAtoi(s, what string) uint16 {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		log.Fatalf("%s: %v", what, err)
	}
	return uint16(n)
}
