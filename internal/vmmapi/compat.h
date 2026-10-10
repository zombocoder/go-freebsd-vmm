/*-
 * SPDX-License-Identifier: BSD-2-Clause
 *
 * Copyright (c) 2026 go-freebsd-vmm contributors
 *
 * Thin compatibility layer between cgo and libvmmapi.
 *
 * This layer exists for three reasons and no others:
 *
 *   1. errno capture.  The Go runtime makes no guarantee that errno
 *	survives the return path of a cgo call, so every wrapper reads it
 *	in the statement immediately following the libvmmapi call.
 *   2. stable call shapes.  cgo is happier with a small number of
 *	regular signatures than with the full variety of libvmmapi.
 *   3. collapsing loops that would otherwise become one cgo transition
 *	per iteration (statistics, guest memory map enumeration).
 *
 * It must not grow into a second implementation of libvmmapi.
 */

#ifndef _GOVMM_COMPAT_H_
#define	_GOVMM_COMPAT_H_

#include <sys/types.h>
#include <sys/param.h>
#include <sys/cpuset.h>
#include <sys/mman.h>
#include <sys/time.h>

#include <machine/vmm.h>
#include <machine/vmm_dev.h>
#include <machine/vmm_snapshot.h>

#include <stddef.h>
#include <stdint.h>

#include <vmmapi.h>

/*
 * Length of a single statistics description as returned by the kernel.
 * Asserted against the real ABI below so a future bump is a build error
 * rather than a truncation.
 */
#define	GOVMM_STAT_DESC_LEN	128

_Static_assert(sizeof(((struct vm_stat_desc *)0)->desc) ==
    GOVMM_STAT_DESC_LEN, "vm_stat_desc::desc size changed");

/*
 * Result carriers.  'rc' mirrors the libvmmapi return value and 'err' is
 * the captured errno, meaningful only when the call failed.
 */
struct govmm_result {
	int	rc;
	int	err;
};

struct govmm_ptr_result {
	void	*ptr;
	int	 err;
};

struct govmm_u64_result {
	uint64_t	val;
	int		rc;
	int		err;
};

/*
 * One entry of the guest address space map, flattened for Go.
 */
struct govmm_memmap {
	uint64_t	gpa;
	uint64_t	segoff;
	uint64_t	len;
	int32_t		segid;
	int32_t		prot;
	uint32_t	flags;
	uint32_t	_pad;
};

/*
 * Instruction bytes carried by an instruction emulation exit.
 */
#define	GOVMM_INST_SIZE		VIE_INST_SIZE

/*
 * A VM exit, flattened.
 *
 * struct vm_exit is a union of fourteen payloads, two of which contain
 * bit-fields and one a whole struct vie.  None of that is addressable
 * from Go, so the decode happens here and Go only ever sees scalars.
 * Fields are grouped by the exit code that makes them meaningful;
 * everything else is zero.
 */
struct govmm_exit {
	uint32_t	code;		/* enum vm_exitcode */
	int32_t		inst_length;	/* 0 means unknown */
	uint64_t	rip;

	/* INOUT and INOUT_STR */
	uint32_t	inout_eax;
	uint16_t	inout_port;
	uint8_t		inout_bytes;
	uint8_t		inout_in;
	uint8_t		inout_string;
	uint8_t		inout_rep;

	/* INOUT_STR only */
	uint64_t	str_rflags;
	uint64_t	str_cr0;
	uint64_t	str_index;
	uint64_t	str_count;
	uint64_t	str_seg_base;
	uint32_t	str_seg_limit;
	uint32_t	str_seg_access;
	int32_t		str_addrsize;
	int32_t		str_seg_name;	/* enum vm_reg_name */

	/* PAGING and INST_EMUL */
	uint64_t	gpa;
	uint64_t	gla;
	uint64_t	cs_base;
	int32_t		fault_type;	/* PAGING */
	int32_t		cs_d;		/* INST_EMUL: CS.D */
	uint8_t		inst[GOVMM_INST_SIZE];
	uint8_t		inst_num_valid;

	/*
	 * Guest paging state, meaningful for INST_EMUL, INOUT_STR,
	 * TASK_SWITCH and DB.
	 */
	uint64_t	paging_cr3;
	int32_t		paging_cpl;
	int32_t		paging_cpu_mode;	/* enum vm_cpu_mode */
	int32_t		paging_mode;		/* enum vm_paging_mode */

	/* VMX */
	uint64_t	vmx_exit_qualification;
	uint32_t	vmx_exit_reason;
	int32_t		vmx_status;
	int32_t		vmx_inst_type;
	int32_t		vmx_inst_error;

	/* SVM */
	uint64_t	svm_exitcode;
	uint64_t	svm_exitinfo1;
	uint64_t	svm_exitinfo2;

	/* RDMSR and WRMSR */
	uint64_t	msr_wval;
	uint32_t	msr_code;	/* ecx value */

	/* SPINUP_AP */
	uint64_t	spinup_rip;
	int32_t		spinup_vcpu;

	/* HLT */
	uint64_t	hlt_rflags;
	uint64_t	hlt_intr_status;

	/* IOAPIC_EOI */
	int32_t		ioapic_vector;

	/* SUSPENDED */
	int32_t		suspend_how;	/* enum vm_suspend_how */

	/* IPI */
	uint32_t	ipi_mode;
	uint8_t		ipi_vector;

	/* DB */
	int32_t		dbg_trace_trap;
	int32_t		dbg_pushf_intercept;
	int32_t		dbg_tf_shadow_val;

	/* BPT */
	int32_t		bpt_inst_length;

	/* TASK_SWITCH */
	uint32_t	ts_errcode;
	uint16_t	ts_tsssel;
	int32_t		ts_ext;
	int32_t		ts_errcode_valid;
	int32_t		ts_reason;	/* enum task_switch_reason */
};

/*
 * Guest paging state on the way in, for the address translation and
 * guest copy calls.
 */
struct govmm_paging {
	uint64_t	cr3;
	int32_t		cpl;
	int32_t		cpu_mode;	/* enum vm_cpu_mode */
	int32_t		paging_mode;	/* enum vm_paging_mode */
	int32_t		_pad;
};

__BEGIN_DECLS

/* Build-time facts. */
const char	*govmm_api_version_string(void);
int		 govmm_cpuset_words(void);

/* VM context lifecycle. */
struct govmm_result	govmm_vm_create(const char *name);
struct govmm_ptr_result	govmm_vm_open(const char *name);
void			govmm_vm_close(struct vmctx *ctx);
void			govmm_vm_destroy(struct vmctx *ctx);
struct govmm_result	govmm_vm_reinit(struct vmctx *ctx);
struct govmm_result	govmm_vm_suspend(struct vmctx *ctx, int how);
const char		*govmm_vm_get_name(struct vmctx *ctx);

/* vCPU lifecycle. */
struct govmm_ptr_result	govmm_vcpu_open(struct vmctx *ctx, int vcpuid);
void			govmm_vcpu_close(struct vcpu *vcpu);
int			govmm_vcpu_id(struct vcpu *vcpu);
struct govmm_result	govmm_vcpu_reset(struct vcpu *vcpu);
struct govmm_result	govmm_vm_activate_cpu(struct vcpu *vcpu);
struct govmm_result	govmm_vm_suspend_cpu(struct vcpu *vcpu);
struct govmm_result	govmm_vm_resume_cpu(struct vcpu *vcpu);
struct govmm_result	govmm_vm_restart_instruction(struct vcpu *vcpu);
struct govmm_result	govmm_vm_suspend_all_cpus(struct vmctx *ctx);
struct govmm_result	govmm_vm_resume_all_cpus(struct vmctx *ctx);

/* CPU sets, copied out as a little-endian array of 64-bit words. */
struct govmm_result	govmm_vm_active_cpus(struct vmctx *ctx,
			    uint64_t *words, int nwords);
struct govmm_result	govmm_vm_suspended_cpus(struct vmctx *ctx,
			    uint64_t *words, int nwords);
struct govmm_result	govmm_vm_debug_cpus(struct vmctx *ctx,
			    uint64_t *words, int nwords);

/* CPU topology. */
struct govmm_result	govmm_vm_get_topology(struct vmctx *ctx,
			    uint16_t *sockets, uint16_t *cores,
			    uint16_t *threads, uint16_t *maxcpus);
struct govmm_result	govmm_vm_set_topology(struct vmctx *ctx,
			    uint16_t sockets, uint16_t cores,
			    uint16_t threads, uint16_t maxcpus);

/* Registers. */
struct govmm_result	govmm_vm_get_register(struct vcpu *vcpu, int reg,
			    uint64_t *val);
struct govmm_result	govmm_vm_set_register(struct vcpu *vcpu, int reg,
			    uint64_t val);
struct govmm_result	govmm_vm_get_register_set(struct vcpu *vcpu,
			    unsigned int count, const int *regnums,
			    uint64_t *vals);
struct govmm_result	govmm_vm_set_register_set(struct vcpu *vcpu,
			    unsigned int count, const int *regnums,
			    uint64_t *vals);
struct govmm_result	govmm_vm_get_desc(struct vcpu *vcpu, int reg,
			    uint64_t *base, uint32_t *limit,
			    uint32_t *access);
struct govmm_result	govmm_vm_set_desc(struct vcpu *vcpu, int reg,
			    uint64_t base, uint32_t limit, uint32_t access);

/* Guest memory. */
struct govmm_result	govmm_vm_setup_memory(struct vmctx *ctx, size_t len,
			    int style);
size_t			govmm_vm_get_lowmem_size(struct vmctx *ctx);
size_t			govmm_vm_get_highmem_size(struct vmctx *ctx);
uint32_t		govmm_vm_get_lowmem_limit(struct vmctx *ctx);
void			govmm_vm_set_lowmem_limit(struct vmctx *ctx,
			    uint32_t limit);
int			govmm_vm_get_memflags(struct vmctx *ctx);
void			govmm_vm_set_memflags(struct vmctx *ctx, int flags);
struct govmm_ptr_result	govmm_vm_map_gpa(struct vmctx *ctx, uint64_t gpa,
			    size_t len);
struct govmm_u64_result	govmm_vm_rev_map_gpa(struct vmctx *ctx, void *addr);
struct govmm_result	govmm_vm_get_memseg(struct vmctx *ctx, int segid,
			    size_t *lenp, char *name, size_t namesz);
struct govmm_ptr_result	govmm_vm_create_devmem(struct vmctx *ctx, int segid,
			    const char *name, size_t len);
struct govmm_result	govmm_vm_mmap_memseg(struct vmctx *ctx, uint64_t gpa,
			    int segid, uint64_t segoff, size_t len, int prot);
struct govmm_result	govmm_vm_munmap_memseg(struct vmctx *ctx,
			    uint64_t gpa, size_t len);

/*
 * Enumerate the whole guest address space in one transition.  On ENOSPC
 * '*count' holds the number of entries actually present.
 */
struct govmm_result	govmm_vm_memmaps(struct vmctx *ctx,
			    struct govmm_memmap *out, int max, int *count);

/* Capabilities. */
struct govmm_result	govmm_vm_get_capability(struct vcpu *vcpu, int cap,
			    int *val);
struct govmm_result	govmm_vm_set_capability(struct vcpu *vcpu, int cap,
			    int val);
int			govmm_vm_capability_name2type(const char *name);
const char		*govmm_vm_capability_type2name(int type);

/*
 * Statistics.  Values and their descriptions are collected in a single
 * transition so that no pointer into libvmmapi's static storage outlives
 * the call.  On ENOSPC '*count' holds the number of entries available.
 */
struct govmm_result	govmm_vm_get_stats(struct vmctx *ctx,
			    struct vcpu *vcpu, uint64_t *vals, char *descs,
			    int max, int *count, int64_t *sec, int64_t *usec);

/*
 * Execution.
 *
 * 'ipi_cpus' receives the destination vCPU mask of an IPI exit, which
 * the kernel copies out separately from the exit payload.
 */
struct govmm_result	govmm_vm_run(struct vcpu *vcpu,
			    struct govmm_exit *out, uint64_t *ipi_cpus,
			    int nwords);

/* Interrupt injection, amd64. */
struct govmm_result	govmm_vm_inject_nmi(struct vcpu *vcpu);
struct govmm_result	govmm_vm_inject_exception(struct vcpu *vcpu,
			    int vector, int errcode_valid, uint32_t errcode,
			    int restart_instruction);
struct govmm_result	govmm_vm_lapic_irq(struct vcpu *vcpu, int vector);
struct govmm_result	govmm_vm_lapic_local_irq(struct vcpu *vcpu,
			    int vector);
struct govmm_result	govmm_vm_lapic_msi(struct vmctx *ctx, uint64_t addr,
			    uint64_t msg);
struct govmm_result	govmm_vm_ioapic_assert_irq(struct vmctx *ctx, int irq);
struct govmm_result	govmm_vm_ioapic_deassert_irq(struct vmctx *ctx,
			    int irq);
struct govmm_result	govmm_vm_ioapic_pulse_irq(struct vmctx *ctx, int irq);
struct govmm_result	govmm_vm_ioapic_pincount(struct vmctx *ctx,
			    int *pincount);
struct govmm_result	govmm_vm_isa_assert_irq(struct vmctx *ctx,
			    int atpic_irq, int ioapic_irq);
struct govmm_result	govmm_vm_isa_deassert_irq(struct vmctx *ctx,
			    int atpic_irq, int ioapic_irq);
struct govmm_result	govmm_vm_isa_pulse_irq(struct vmctx *ctx,
			    int atpic_irq, int ioapic_irq);
struct govmm_result	govmm_vm_isa_set_irq_trigger(struct vmctx *ctx,
			    int atpic_irq, int trigger);
struct govmm_result	govmm_vm_get_intinfo(struct vcpu *vcpu, uint64_t *i1,
			    uint64_t *i2);
struct govmm_result	govmm_vm_set_intinfo(struct vcpu *vcpu,
			    uint64_t exit_intinfo);
struct govmm_result	govmm_vm_get_x2apic_state(struct vcpu *vcpu,
			    int *state);
struct govmm_result	govmm_vm_set_x2apic_state(struct vcpu *vcpu,
			    int state);
struct govmm_result	govmm_vm_get_hpet_capabilities(struct vmctx *ctx,
			    uint32_t *capabilities);
struct govmm_result	govmm_vm_apicid2vcpu(struct vmctx *ctx, int apicid);
struct govmm_result	govmm_vm_readwrite_kernemu_device(struct vcpu *vcpu,
			    uint64_t gpa, int write, int size,
			    uint64_t *value);

/* PCI passthrough. */
struct govmm_result	govmm_vm_assign_pptdev(struct vmctx *ctx, int bus,
			    int slot, int func);
struct govmm_result	govmm_vm_unassign_pptdev(struct vmctx *ctx, int bus,
			    int slot, int func);
struct govmm_result	govmm_vm_map_pptdev_mmio(struct vmctx *ctx, int bus,
			    int slot, int func, uint64_t gpa, size_t len,
			    uint64_t hpa);
struct govmm_result	govmm_vm_unmap_pptdev_mmio(struct vmctx *ctx, int bus,
			    int slot, int func, uint64_t gpa, size_t len);
struct govmm_result	govmm_vm_setup_pptdev_msi(struct vmctx *ctx, int bus,
			    int slot, int func, uint64_t addr, uint64_t msg,
			    int numvec);
struct govmm_result	govmm_vm_setup_pptdev_msix(struct vmctx *ctx, int bus,
			    int slot, int func, int idx, uint64_t addr,
			    uint64_t msg, uint32_t vector_control);
struct govmm_result	govmm_vm_disable_pptdev_msix(struct vmctx *ctx,
			    int bus, int slot, int func);

/* RTC, amd64. */
struct govmm_result	govmm_vm_rtc_read(struct vmctx *ctx, int offset,
			    uint8_t *value);
struct govmm_result	govmm_vm_rtc_write(struct vmctx *ctx, int offset,
			    uint8_t value);
struct govmm_result	govmm_vm_rtc_gettime(struct vmctx *ctx,
			    int64_t *secs);
struct govmm_result	govmm_vm_rtc_settime(struct vmctx *ctx, int64_t secs);

/* Address translation. */
struct govmm_result	govmm_vm_gla2gpa(struct vcpu *vcpu,
			    const struct govmm_paging *paging, uint64_t gla,
			    int prot, uint64_t *gpa, int *fault);
struct govmm_result	govmm_vm_gla2gpa_nofault(struct vcpu *vcpu,
			    const struct govmm_paging *paging, uint64_t gla,
			    int prot, uint64_t *gpa, int *fault);
struct govmm_result	govmm_vm_get_gpa_pmap(struct vmctx *ctx, uint64_t gpa,
			    uint64_t *pte, int *num);

/*
 * Guest virtual memory access.
 *
 * Translation, copy and teardown happen in one transition.  The caller's
 * buffer is host memory; 'fault' is set when the guest took a page fault
 * instead, in which case an exception has been injected and nothing was
 * copied.
 */
struct govmm_result	govmm_vm_read_guest_virtual(struct vcpu *vcpu,
			    const struct govmm_paging *paging, uint64_t gla,
			    void *dst, size_t len, int *fault);
struct govmm_result	govmm_vm_write_guest_virtual(struct vcpu *vcpu,
			    const struct govmm_paging *paging, uint64_t gla,
			    const void *src, size_t len, int *fault);

/* Snapshots and Capsicum. */
struct govmm_result	govmm_vm_snapshot_req(struct vmctx *ctx,
			    const char *dev_name, int dev_req, int op,
			    void *buf, size_t buflen, size_t *used);
struct govmm_result	govmm_vm_restore_time(struct vmctx *ctx);
struct govmm_result	govmm_vm_limit_rights(struct vmctx *ctx);

__END_DECLS

#endif /* !_GOVMM_COMPAT_H_ */
