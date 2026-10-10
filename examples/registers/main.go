//go:build freebsd && amd64

// Command registers dumps a vCPU's registers.
//
//	registers vm01 0
//
// The whole set is read with one call, which costs one cgo transition
// and one ioctl rather than one of each per register.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	vmm "github.com/zombocoder/go-freebsd-vmm"
)

// dump is the set a VM exit handler usually wants to see.
var dump = []vmm.Register{
	vmm.RegRAX, vmm.RegRBX, vmm.RegRCX, vmm.RegRDX,
	vmm.RegRSI, vmm.RegRDI, vmm.RegRBP, vmm.RegRSP,
	vmm.RegR8, vmm.RegR9, vmm.RegR10, vmm.RegR11,
	vmm.RegR12, vmm.RegR13, vmm.RegR14, vmm.RegR15,
	vmm.RegRIP, vmm.RegRFLAGS,
	vmm.RegCR0, vmm.RegCR2, vmm.RegCR3, vmm.RegCR4,
	vmm.RegEFER,
}

// descriptors are the segment registers whose base, limit and access
// bits a plain register value does not carry.
var descriptors = []vmm.Register{
	vmm.RegCS, vmm.RegDS, vmm.RegES, vmm.RegSS,
	vmm.RegFS, vmm.RegGS, vmm.RegGDTR, vmm.RegIDTR,
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("registers: ")

	if len(os.Args) != 3 {
		log.Fatalf("usage: %s <vm-name> <vcpu>", os.Args[0])
	}
	id, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("vcpu: %v", err)
	}

	vm, err := vmm.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer vm.Close()

	cpu, err := vm.VCPU(id)
	if err != nil {
		log.Fatal(err)
	}
	defer cpu.Close()

	values, err := cpu.Registers(dump)
	if err != nil {
		log.Fatal(err)
	}
	for _, rv := range values {
		fmt.Printf("%-7s %#018x\n", rv.Register, rv.Value)
	}

	fmt.Println()
	for _, reg := range descriptors {
		desc, err := cpu.SegmentDescriptor(reg)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-7s %v\n", reg, desc)
	}
}
