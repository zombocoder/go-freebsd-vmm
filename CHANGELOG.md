# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
The API may change until v1.0.0.

## [Unreleased]

### Added

Everything below is the first working slice of the library: the cgo bridge,
the ownership and error models, and the VMM primitives that do not require
entering the guest.

- **VM lifecycle.** `Create`, `Open`, `OpenWithFlags`, `CreateAndOpen`,
  `Close`, `Destroy`, `Name`, `Reinitialize`, `Suspend`.
  `OpenCreate`, `OpenReinit` and `OpenDestroyOnClose` are built from the
  stable primitives rather than `vm_openf()`, which FreeBSD 14 does not have.
- **vCPU lifecycle.** `VM.VCPU`, `Close`, `ID`, `Reset`, `Activate`,
  `Suspend`, `Resume`, `RestartInstruction`, and `VM.SuspendAllCPUs` /
  `ResumeAllCPUs`. vCPU ids are range-checked against the VM topology,
  because `vm_vcpu_open(3)` issues no ioctl and cannot reject a bad one.
- **CPU state.** `ActiveCPUs`, `SuspendedCPUs`, `DebugCPUs`, returning
  `CPUSet` with `Contains`, `IDs`, `Len` and `String`. `cpuset_t` is not
  exposed.
- **CPU topology.** `Topology` and `SetTopology`, with the obviously invalid
  configurations rejected before they reach the kernel.
- **Registers.** `Register` and `SetRegister`, the bulk `Registers` and
  `SetRegisters`, and `SegmentDescriptor` / `SetSegmentDescriptor`. The full
  amd64 register set is taken from `enum vm_reg_name` at compile time.
- **Guest memory.** `SetupMemory`, `LowMemorySize`, `HighMemorySize`,
  `HighMemoryBase`, `LowMemoryLimit`, `SetLowMemoryLimit`, `MemoryFlags`,
  `SetMemoryFlags`, `MapGuestMemory`, `ReverseMap`, `MemorySegment`,
  `CreateDeviceMemory`, `MapMemorySegment`, `UnmapMemorySegment` and
  `MemoryMappings`.
- **Capabilities.** `VCPU.Capability`, `SetCapability`, `CapabilityByName`
  and `Capability.String`, with names resolved by `libvmmapi` rather than
  from a table here.
- **Statistics.** `VCPU.Stats` and `StatsSnapshot`, serialised across the
  process and copied out of `libvmmapi`'s static storage before returning.
- **Errors.** `*Error` carrying the operation, VM, vCPU and `errno`, so that
  `errors.Is(err, syscall.ENOENT)` works. Sentinels `ErrClosed`,
  `ErrDestroyed`, `ErrInvalidArgument`, `ErrUnsupported`.
- **Version and features.** `APIVersion`, `Version`, `Feature`, `Supports`
  and `Features`.
- **Guest execution.** `VCPU.Run` and `VCPU.RunLoop`, with every one of the
  26 exit reasons decoded into typed payloads: `IOExit`, `StringIOExit`,
  `PagingExit`, `InstEmulExit`, `MSRExit`, `HLTExit`, `IPIExit`,
  `SuspendedExit`, `SpinupAPExit`, `IOAPICEOIExit`, `TaskSwitchExit`,
  `DebugExit`, `BreakpointExit`, `VMXExit` and `SVMExit`. An unknown reason
  keeps its numeric value rather than breaking. `ExitCode.Spurious`
  recognises the exits that mean nothing happened.
- **Interrupts, amd64.** `InjectNMI`, `InjectException`, `LAPICIRQ`,
  `LAPICLocalIRQ`, `LAPICMSI`, `IOAPICAssert` / `Deassert` / `Pulse`,
  `IOAPICPinCount`, `ISAAssert` / `Deassert` / `Pulse`, `ISASetIRQTrigger`,
  `IntInfo` and `SetIntInfo`, `X2APICState` and `SetX2APICState`,
  `HPETCapabilities`, `APICIDToVCPU`, `KernelEmulatedDevice`.
- **PCI passthrough.** `PCIDevice` with validation, a stable
  `bus/slot/function` `String`, and `ParsePCIDevice`. `AssignPassthrough`,
  `UnassignPassthrough`, `MapPassthroughMMIO`, `UnmapPassthroughMMIO`,
  `SetupPassthroughMSI`, `SetupPassthroughMSIX`, `DisablePassthroughMSIX`.
- **RTC, amd64.** `RTCRead`, `RTCWrite`, `RTCTime`, `SetRTCTime`.
- **Address translation.** `GuestLinearToPhysical` and its non-faulting
  variant, `GuestPageMap`, and `PagingState` with `CPUMode` and
  `PagingMode`. A guest page fault is reported as `ErrGuestFault` rather
  than as a failure of the call.
- **Guest memory through linear addresses.** `ReadGuestVirtual` and
  `WriteGuestVirtual`, which translate, copy and tear down in one kernel
  call.
- **Snapshots, experimental.** `SnapshotSave`, `SnapshotRestore`,
  `SnapshotMetadata`, `SnapshotStructure`, `RestoreTime`. These cover what
  vmm(4) owns and do not amount to a restorable virtual machine; see the
  documentation on `SnapshotSave`.
- **Capsicum.** `LimitRights`.
- **Tests.** 114 pure-Go tests that need no kernel, covering argument
  validation, error wrapping, version decoding, CPU set arithmetic, the
  closed-handle state machine and exit decoding against synthetic bridge
  output. Integration tests under `tests/` drive a real kernel and skip with
  the reason when the host cannot host a VM. Level 3 tests under
  `tests/privileged`, behind the `vmm_privileged` tag, execute a five-byte
  real-mode payload and check the exits it produces. Benchmarks cover the
  single-register and bulk-register paths, the statistics path and exit
  decoding.
- **Examples.** `create`, `inspect`, `topology`, `registers`, `memory`,
  `stats`, `run`.

### Notes on the FreeBSD side

Several behaviours of `libvmmapi` shape the API and are worth recording:

- `vm_destroy(3)` closes the descriptor, destroys the VM and frees the
  context. `vm_close(3)` also frees it. `Close` and `Destroy` are therefore
  idempotent and mutually exclusive, so that a deferred `Close` next to an
  explicit `Destroy` cannot double free.
- `vm_setup_memory(3)` asserts that the mapping style is `VM_MMAP_ALL` on
  FreeBSD 14. `SetupMemory` refuses the other styles with `ErrUnsupported`
  rather than letting the assertion abort the process.
- `vm_get_stat_desc(3)` returns a pointer into process-global static
  storage, which is the genuinely thread-unsafe part of the statistics path.
  Values and descriptions are gathered in one C call under a lock.
- `VMMAPI_VERSION` is a leading-zero literal, so C reads it as octal.
  `APIVersion` decodes the digits of the macro, not its value.
- `vm_destroy(3)` returns void, so a kernel-side destroy failure is not
  observable and `Destroy` cannot report one.
- `vm_copy_setup(3)` asserts that it has an iovec left for every page it
  touches, so a short array would abort the process. The bridge walks guest
  virtual copies in bounded chunks with spare entries for a partial page at
  each end.
- The kernel leaves the guest whenever the calling thread has an AST
  pending, a signal included, reporting `VM_EXITCODE_BOGUS`. Because the Go
  runtime signals its threads routinely, these exits are common here, which
  is why `ExitCode.Spurious` exists.
- vmm(4) binds a vCPU to the calling CPU only for the length of one
  `VM_RUN` and keeps no record of which thread entered the guest between
  calls, so thread affinity is not a correctness requirement. `RunLoop`
  pins anyway, to give each vCPU a dedicated host thread as bhyve does.
- FreeBSD's instruction emulator is a kernel source file that bhyve
  compiles into itself; libvmmapi does not export it. Emulated MMIO
  therefore arrives as instruction bytes to decode, and decoding them is
  out of scope.
- `vm_set_topology(9)` declares its maxcpus parameter `__unused`. A VM's
  vCPU count is fixed when it is created, from the `hw.vmm.maxcpu`
  tunable, and `SetTopology` can only change how those vCPUs are arranged.
  `Topology.MaxCPUs` is therefore read-only, and `SetTopology` checks
  sockets x cores x threads against it so the limit is named rather than
  arriving as a bare EINVAL.
- A guest that executes HLT with interrupts disabled is parking its vCPU,
  and once every active vCPU has, vmm(4) halts the whole VM. A
  single-vCPU guest halting therefore reports `ExitSuspended` carrying
  `SuspendHalt`, not `ExitHLT`.
- A suspended vCPU cannot be entered again: `vm_run(2)` checks it against
  the VM's suspended set and reports EINVAL. `Reinitialize` is the way
  back.
- Snapshot support is a kernel build option. Without `BHYVE_SNAPSHOT`
  there is no ioctl behind the snapshot calls and they report ENOTTY,
  which no amount of compile-time detection can predict, so
  `Supports(FeatureSnapshot)` answers for the entry points rather than
  for the kernel.

[Unreleased]: https://github.com/zombocoder/go-freebsd-vmm/commits/master
