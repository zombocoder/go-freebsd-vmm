/*-
 * SPDX-License-Identifier: BSD-2-Clause
 *
 * Copyright (c) 2026 go-freebsd-vmm contributors
 *
 * See compat.h for what this layer is and is not allowed to do.
 */

#include <sys/types.h>
#include <sys/uio.h>

#include <errno.h>
#include <stdbool.h>
#include <string.h>

#include "compat.h"

#define	GOVMM_STRINGIFY_(x)	#x
#define	GOVMM_STRINGIFY(x)	GOVMM_STRINGIFY_(x)

/*
 * Upper bound on guest address space map entries walked by
 * govmm_vm_memmaps().  Purely a runaway guard; real guests have a
 * handful.
 */
#define	GOVMM_MEMMAP_MAX	4096

/*
 * Several libvmmapi helpers are pure address arithmetic over the context
 * and report failure with a sentinel return value without touching
 * errno.  Report EFAULT for those so the caller still gets a usable
 * errno rather than a stale one.
 */
#define	GOVMM_ENO_SENTINEL	EFAULT

/* Build-time facts ------------------------------------------------------ */

const char *
govmm_api_version_string(void)
{

	return (GOVMM_STRINGIFY(VMMAPI_VERSION));
}

int
govmm_cpuset_words(void)
{

	return ((int)((sizeof(cpuset_t) + sizeof(uint64_t) - 1) /
	    sizeof(uint64_t)));
}

/* VM context lifecycle -------------------------------------------------- */

struct govmm_result
govmm_vm_create(const char *name)
{
	struct govmm_result r;

	r.rc = vm_create(name);
	r.err = errno;
	return (r);
}

struct govmm_ptr_result
govmm_vm_open(const char *name)
{
	struct govmm_ptr_result r;

	r.ptr = vm_open(name);
	r.err = errno;
	return (r);
}

void
govmm_vm_close(struct vmctx *ctx)
{

	vm_close(ctx);
}

void
govmm_vm_destroy(struct vmctx *ctx)
{

	vm_destroy(ctx);
}

struct govmm_result
govmm_vm_reinit(struct vmctx *ctx)
{
	struct govmm_result r;

	r.rc = vm_reinit(ctx);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_suspend(struct vmctx *ctx, int how)
{
	struct govmm_result r;

	r.rc = vm_suspend(ctx, (enum vm_suspend_how)how);
	r.err = errno;
	return (r);
}

const char *
govmm_vm_get_name(struct vmctx *ctx)
{

	return (vm_get_name(ctx));
}

/* vCPU lifecycle -------------------------------------------------------- */

struct govmm_ptr_result
govmm_vcpu_open(struct vmctx *ctx, int vcpuid)
{
	struct govmm_ptr_result r;

	r.ptr = vm_vcpu_open(ctx, vcpuid);
	r.err = (r.ptr == NULL) ? ENOMEM : 0;
	return (r);
}

void
govmm_vcpu_close(struct vcpu *vcpu)
{

	vm_vcpu_close(vcpu);
}

int
govmm_vcpu_id(struct vcpu *vcpu)
{

	return (vcpu_id(vcpu));
}

struct govmm_result
govmm_vcpu_reset(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vcpu_reset(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_activate_cpu(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vm_activate_cpu(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_suspend_cpu(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vm_suspend_cpu(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_resume_cpu(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vm_resume_cpu(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_restart_instruction(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vm_restart_instruction(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_suspend_all_cpus(struct vmctx *ctx)
{
	struct govmm_result r;

	r.rc = vm_suspend_all_cpus(ctx);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_resume_all_cpus(struct vmctx *ctx)
{
	struct govmm_result r;

	r.rc = vm_resume_all_cpus(ctx);
	r.err = errno;
	return (r);
}

/* CPU sets -------------------------------------------------------------- */

/*
 * Copy a cpuset_t into the caller's word array.  The bitset layout is
 * already a little-endian array of machine words on every architecture
 * FreeBSD supports, so a byte copy is the whole conversion.
 */
static void
govmm_cpuset_copyout(const cpuset_t *set, uint64_t *words, int nwords)
{
	size_t avail, have;

	avail = (size_t)nwords * sizeof(uint64_t);
	have = sizeof(*set);
	if (have > avail)
		have = avail;
	memset(words, 0, avail);
	memcpy(words, set, have);
}

struct govmm_result
govmm_vm_active_cpus(struct vmctx *ctx, uint64_t *words, int nwords)
{
	struct govmm_result r;
	cpuset_t set;

	CPU_ZERO(&set);
	r.rc = vm_active_cpus(ctx, &set);
	r.err = errno;
	if (r.rc == 0)
		govmm_cpuset_copyout(&set, words, nwords);
	return (r);
}

struct govmm_result
govmm_vm_suspended_cpus(struct vmctx *ctx, uint64_t *words, int nwords)
{
	struct govmm_result r;
	cpuset_t set;

	CPU_ZERO(&set);
	r.rc = vm_suspended_cpus(ctx, &set);
	r.err = errno;
	if (r.rc == 0)
		govmm_cpuset_copyout(&set, words, nwords);
	return (r);
}

struct govmm_result
govmm_vm_debug_cpus(struct vmctx *ctx, uint64_t *words, int nwords)
{
	struct govmm_result r;
	cpuset_t set;

	CPU_ZERO(&set);
	r.rc = vm_debug_cpus(ctx, &set);
	r.err = errno;
	if (r.rc == 0)
		govmm_cpuset_copyout(&set, words, nwords);
	return (r);
}

/* CPU topology ---------------------------------------------------------- */

struct govmm_result
govmm_vm_get_topology(struct vmctx *ctx, uint16_t *sockets, uint16_t *cores,
    uint16_t *threads, uint16_t *maxcpus)
{
	struct govmm_result r;

	r.rc = vm_get_topology(ctx, sockets, cores, threads, maxcpus);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_topology(struct vmctx *ctx, uint16_t sockets, uint16_t cores,
    uint16_t threads, uint16_t maxcpus)
{
	struct govmm_result r;

	r.rc = vm_set_topology(ctx, sockets, cores, threads, maxcpus);
	r.err = errno;
	return (r);
}

/* Registers ------------------------------------------------------------- */

struct govmm_result
govmm_vm_get_register(struct vcpu *vcpu, int reg, uint64_t *val)
{
	struct govmm_result r;

	r.rc = vm_get_register(vcpu, reg, val);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_register(struct vcpu *vcpu, int reg, uint64_t val)
{
	struct govmm_result r;

	r.rc = vm_set_register(vcpu, reg, val);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_register_set(struct vcpu *vcpu, unsigned int count,
    const int *regnums, uint64_t *vals)
{
	struct govmm_result r;

	r.rc = vm_get_register_set(vcpu, count, regnums, vals);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_register_set(struct vcpu *vcpu, unsigned int count,
    const int *regnums, uint64_t *vals)
{
	struct govmm_result r;

	r.rc = vm_set_register_set(vcpu, count, regnums, vals);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_desc(struct vcpu *vcpu, int reg, uint64_t *base, uint32_t *limit,
    uint32_t *access)
{
	struct govmm_result r;

	r.rc = vm_get_desc(vcpu, reg, base, limit, access);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_desc(struct vcpu *vcpu, int reg, uint64_t base, uint32_t limit,
    uint32_t access)
{
	struct govmm_result r;

	r.rc = vm_set_desc(vcpu, reg, base, limit, access);
	r.err = errno;
	return (r);
}

/* Guest memory ---------------------------------------------------------- */

struct govmm_result
govmm_vm_setup_memory(struct vmctx *ctx, size_t len, int style)
{
	struct govmm_result r;

	r.rc = vm_setup_memory(ctx, len, (enum vm_mmap_style)style);
	r.err = errno;
	return (r);
}

size_t
govmm_vm_get_lowmem_size(struct vmctx *ctx)
{

	return (vm_get_lowmem_size(ctx));
}

size_t
govmm_vm_get_highmem_size(struct vmctx *ctx)
{

	return (vm_get_highmem_size(ctx));
}

uint32_t
govmm_vm_get_lowmem_limit(struct vmctx *ctx)
{

	return (vm_get_lowmem_limit(ctx));
}

void
govmm_vm_set_lowmem_limit(struct vmctx *ctx, uint32_t limit)
{

	vm_set_lowmem_limit(ctx, limit);
}

int
govmm_vm_get_memflags(struct vmctx *ctx)
{

	return (vm_get_memflags(ctx));
}

void
govmm_vm_set_memflags(struct vmctx *ctx, int flags)
{

	vm_set_memflags(ctx, flags);
}

struct govmm_ptr_result
govmm_vm_map_gpa(struct vmctx *ctx, uint64_t gpa, size_t len)
{
	struct govmm_ptr_result r;

	errno = 0;
	r.ptr = vm_map_gpa(ctx, gpa, len);
	r.err = errno;
	if (r.ptr == NULL && r.err == 0)
		r.err = GOVMM_ENO_SENTINEL;
	return (r);
}

struct govmm_u64_result
govmm_vm_rev_map_gpa(struct vmctx *ctx, void *addr)
{
	struct govmm_u64_result r;

	r.val = vm_rev_map_gpa(ctx, addr);
	if (r.val == (uint64_t)-1) {
		r.rc = -1;
		r.err = GOVMM_ENO_SENTINEL;
	} else {
		r.rc = 0;
		r.err = 0;
	}
	return (r);
}

struct govmm_result
govmm_vm_get_memseg(struct vmctx *ctx, int segid, size_t *lenp, char *name,
    size_t namesz)
{
	struct govmm_result r;

	r.rc = vm_get_memseg(ctx, segid, lenp, name, namesz);
	r.err = errno;
	return (r);
}

struct govmm_ptr_result
govmm_vm_create_devmem(struct vmctx *ctx, int segid, const char *name,
    size_t len)
{
	struct govmm_ptr_result r;

	errno = 0;
	r.ptr = vm_create_devmem(ctx, segid, name, len);
	r.err = errno;
	if (r.ptr == MAP_FAILED) {
		r.ptr = NULL;
		if (r.err == 0)
			r.err = GOVMM_ENO_SENTINEL;
	}
	return (r);
}

struct govmm_result
govmm_vm_mmap_memseg(struct vmctx *ctx, uint64_t gpa, int segid,
    uint64_t segoff, size_t len, int prot)
{
	struct govmm_result r;

	r.rc = vm_mmap_memseg(ctx, gpa, segid, (vm_ooffset_t)segoff, len,
	    prot);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_munmap_memseg(struct vmctx *ctx, uint64_t gpa, size_t len)
{
	struct govmm_result r;

	r.rc = vm_munmap_memseg(ctx, gpa, len);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_memmaps(struct vmctx *ctx, struct govmm_memmap *out, int max,
    int *count)
{
	struct govmm_result r;
	vm_paddr_t gpa;
	vm_ooffset_t segoff;
	size_t len;
	int flags, n, prot, segid;

	r.rc = 0;
	r.err = 0;
	*count = 0;

	gpa = 0;
	for (n = 0; n < GOVMM_MEMMAP_MAX; n++) {
		if (vm_mmap_getnext(ctx, &gpa, &segid, &segoff, &len, &prot,
		    &flags) != 0)
			break;
		/*
		 * A zero-length range would stall the walk; treat it as the
		 * end of the map rather than looping forever.
		 */
		if (len == 0)
			break;
		if (n < max) {
			out[n].gpa = gpa;
			out[n].segoff = (uint64_t)segoff;
			out[n].len = len;
			out[n].segid = segid;
			out[n].prot = prot;
			out[n].flags = (uint32_t)flags;
			out[n]._pad = 0;
		}
		gpa += len;
	}

	*count = n;
	if (n > max) {
		r.rc = -1;
		r.err = ENOSPC;
	}
	return (r);
}

/* Capabilities ---------------------------------------------------------- */

struct govmm_result
govmm_vm_get_capability(struct vcpu *vcpu, int cap, int *val)
{
	struct govmm_result r;

	r.rc = vm_get_capability(vcpu, (enum vm_cap_type)cap, val);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_capability(struct vcpu *vcpu, int cap, int val)
{
	struct govmm_result r;

	r.rc = vm_set_capability(vcpu, (enum vm_cap_type)cap, val);
	r.err = errno;
	return (r);
}

int
govmm_vm_capability_name2type(const char *name)
{

	return (vm_capability_name2type(name));
}

const char *
govmm_vm_capability_type2name(int type)
{

	return (vm_capability_type2name(type));
}

/* Statistics ------------------------------------------------------------ */

/*
 * Collect values and descriptions in one go.
 *
 * vm_get_stats() hands back a pointer into a thread-local buffer and
 * vm_get_stat_desc() a pointer into process-global static storage.
 * Neither may outlive this function, so both are copied before return.
 * The caller serialises these calls.
 */
struct govmm_result
govmm_vm_get_stats(struct vmctx *ctx, struct vcpu *vcpu, uint64_t *vals,
    char *descs, int max, int *count, int64_t *sec, int64_t *usec)
{
	struct govmm_result r;
	struct timeval tv;
	const char *desc;
	uint64_t *stats;
	int entries, i;

	r.rc = 0;
	r.err = 0;
	*count = 0;
	*sec = 0;
	*usec = 0;

	entries = 0;
	stats = vm_get_stats(vcpu, &tv, &entries);
	if (stats == NULL) {
		r.rc = -1;
		r.err = errno;
		return (r);
	}

	*count = entries;
	*sec = (int64_t)tv.tv_sec;
	*usec = (int64_t)tv.tv_usec;

	if (entries > max) {
		r.rc = -1;
		r.err = ENOSPC;
		return (r);
	}

	memcpy(vals, stats, (size_t)entries * sizeof(uint64_t));

	for (i = 0; i < entries; i++) {
		desc = vm_get_stat_desc(ctx, i);
		if (desc == NULL)
			descs[(size_t)i * GOVMM_STAT_DESC_LEN] = '\0';
		else
			strlcpy(&descs[(size_t)i * GOVMM_STAT_DESC_LEN], desc,
			    GOVMM_STAT_DESC_LEN);
	}

	return (r);
}

/* Execution ------------------------------------------------------------- */

/*
 * Copy the guest paging state out of an exit payload.
 */
static void
govmm_paging_copyout(struct govmm_exit *out,
    const struct vm_guest_paging *paging)
{

	out->paging_cr3 = paging->cr3;
	out->paging_cpl = paging->cpl;
	out->paging_cpu_mode = (int32_t)paging->cpu_mode;
	out->paging_mode = (int32_t)paging->paging_mode;
}

/*
 * Copy the fields shared by INOUT and INOUT_STR.
 */
static void
govmm_inout_copyout(struct govmm_exit *out, const struct vm_inout *inout)
{

	out->inout_port = inout->port;
	out->inout_bytes = inout->bytes;
	out->inout_in = inout->in;
	out->inout_string = inout->string;
	out->inout_rep = inout->rep;
	out->inout_eax = inout->eax;
}

/*
 * Flatten one struct vm_exit.
 *
 * Only the payload named by the exit code is read; the rest of the union
 * holds whatever the previous exit left there.
 */
static void
govmm_exit_decode(struct govmm_exit *out, const struct vm_exit *vme)
{

	memset(out, 0, sizeof(*out));

	out->code = (uint32_t)vme->exitcode;
	out->inst_length = vme->inst_length;
	out->rip = vme->rip;

	switch (vme->exitcode) {
	case VM_EXITCODE_INOUT:
		govmm_inout_copyout(out, &vme->u.inout);
		break;

	case VM_EXITCODE_INOUT_STR:
		govmm_inout_copyout(out, &vme->u.inout_str.inout);
		govmm_paging_copyout(out, &vme->u.inout_str.paging);
		out->str_rflags = vme->u.inout_str.rflags;
		out->str_cr0 = vme->u.inout_str.cr0;
		out->str_index = vme->u.inout_str.index;
		out->str_count = vme->u.inout_str.count;
		out->str_addrsize = vme->u.inout_str.addrsize;
		out->str_seg_name = (int32_t)vme->u.inout_str.seg_name;
		out->str_seg_base = vme->u.inout_str.seg_desc.base;
		out->str_seg_limit = vme->u.inout_str.seg_desc.limit;
		out->str_seg_access = vme->u.inout_str.seg_desc.access;
		break;

	case VM_EXITCODE_PAGING:
		out->gpa = vme->u.paging.gpa;
		out->fault_type = vme->u.paging.fault_type;
		break;

	case VM_EXITCODE_INST_EMUL:
		out->gpa = vme->u.inst_emul.gpa;
		out->gla = vme->u.inst_emul.gla;
		out->cs_base = vme->u.inst_emul.cs_base;
		out->cs_d = vme->u.inst_emul.cs_d;
		govmm_paging_copyout(out, &vme->u.inst_emul.paging);
		memcpy(out->inst, vme->u.inst_emul.vie.inst,
		    sizeof(out->inst));
		out->inst_num_valid = vme->u.inst_emul.vie.num_valid;
		break;

	case VM_EXITCODE_VMX:
		out->vmx_status = vme->u.vmx.status;
		out->vmx_exit_reason = vme->u.vmx.exit_reason;
		out->vmx_exit_qualification = vme->u.vmx.exit_qualification;
		out->vmx_inst_type = vme->u.vmx.inst_type;
		out->vmx_inst_error = vme->u.vmx.inst_error;
		break;

	case VM_EXITCODE_SVM:
		out->svm_exitcode = vme->u.svm.exitcode;
		out->svm_exitinfo1 = vme->u.svm.exitinfo1;
		out->svm_exitinfo2 = vme->u.svm.exitinfo2;
		break;

	case VM_EXITCODE_RDMSR:
	case VM_EXITCODE_WRMSR:
		out->msr_code = vme->u.msr.code;
		out->msr_wval = vme->u.msr.wval;
		break;

	case VM_EXITCODE_SPINUP_AP:
		out->spinup_vcpu = vme->u.spinup_ap.vcpu;
		out->spinup_rip = vme->u.spinup_ap.rip;
		break;

	case VM_EXITCODE_HLT:
		out->hlt_rflags = vme->u.hlt.rflags;
		out->hlt_intr_status = vme->u.hlt.intr_status;
		break;

	case VM_EXITCODE_IOAPIC_EOI:
		out->ioapic_vector = vme->u.ioapic_eoi.vector;
		break;

	case VM_EXITCODE_SUSPENDED:
		out->suspend_how = (int32_t)vme->u.suspended.how;
		break;

	case VM_EXITCODE_IPI:
		out->ipi_mode = vme->u.ipi.mode;
		out->ipi_vector = vme->u.ipi.vector;
		break;

	case VM_EXITCODE_DB:
		out->dbg_trace_trap = vme->u.dbg.trace_trap;
		out->dbg_pushf_intercept = vme->u.dbg.pushf_intercept;
		out->dbg_tf_shadow_val = vme->u.dbg.tf_shadow_val;
		govmm_paging_copyout(out, &vme->u.dbg.paging);
		break;

	case VM_EXITCODE_BPT:
		out->bpt_inst_length = vme->u.bpt.inst_length;
		break;

	case VM_EXITCODE_TASK_SWITCH:
		out->ts_tsssel = vme->u.task_switch.tsssel;
		out->ts_ext = vme->u.task_switch.ext;
		out->ts_errcode = vme->u.task_switch.errcode;
		out->ts_errcode_valid = vme->u.task_switch.errcode_valid;
		out->ts_reason = (int32_t)vme->u.task_switch.reason;
		govmm_paging_copyout(out, &vme->u.task_switch.paging);
		break;

	default:
		/*
		 * BOGUS, HLT handled above, MTRAP, PAUSE, RENDEZVOUS,
		 * REQIDLE, MONITOR, MWAIT, VMINSN and anything a future
		 * release adds carry no payload we can interpret.  The
		 * code, rip and instruction length are already set.
		 */
		break;
	}
}

struct govmm_result
govmm_vm_run(struct vcpu *vcpu, struct govmm_exit *out, uint64_t *ipi_cpus,
    int nwords)
{
	struct govmm_result r;
	struct vm_exit vme;
	struct vm_run vmrun;
	cpuset_t dmask;

	memset(&vme, 0, sizeof(vme));
	CPU_ZERO(&dmask);

	/*
	 * vcpu_ioctl() writes the vcpuid into the first field of the
	 * argument, so vmrun.cpuid is filled in for us.
	 */
	memset(&vmrun, 0, sizeof(vmrun));
	vmrun.vm_exit = &vme;
	vmrun.cpuset = &dmask;
	vmrun.cpusetsize = sizeof(dmask);

	r.rc = vm_run(vcpu, &vmrun);
	r.err = errno;
	if (r.rc == 0) {
		govmm_exit_decode(out, &vme);
		govmm_cpuset_copyout(&dmask, ipi_cpus, nwords);
	}
	return (r);
}

/* Interrupt injection, amd64 ------------------------------------------- */

struct govmm_result
govmm_vm_inject_nmi(struct vcpu *vcpu)
{
	struct govmm_result r;

	r.rc = vm_inject_nmi(vcpu);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_inject_exception(struct vcpu *vcpu, int vector, int errcode_valid,
    uint32_t errcode, int restart_instruction)
{
	struct govmm_result r;

	r.rc = vm_inject_exception(vcpu, vector, errcode_valid, errcode,
	    restart_instruction);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_lapic_irq(struct vcpu *vcpu, int vector)
{
	struct govmm_result r;

	r.rc = vm_lapic_irq(vcpu, vector);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_lapic_local_irq(struct vcpu *vcpu, int vector)
{
	struct govmm_result r;

	r.rc = vm_lapic_local_irq(vcpu, vector);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_lapic_msi(struct vmctx *ctx, uint64_t addr, uint64_t msg)
{
	struct govmm_result r;

	r.rc = vm_lapic_msi(ctx, addr, msg);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_ioapic_assert_irq(struct vmctx *ctx, int irq)
{
	struct govmm_result r;

	r.rc = vm_ioapic_assert_irq(ctx, irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_ioapic_deassert_irq(struct vmctx *ctx, int irq)
{
	struct govmm_result r;

	r.rc = vm_ioapic_deassert_irq(ctx, irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_ioapic_pulse_irq(struct vmctx *ctx, int irq)
{
	struct govmm_result r;

	r.rc = vm_ioapic_pulse_irq(ctx, irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_ioapic_pincount(struct vmctx *ctx, int *pincount)
{
	struct govmm_result r;

	r.rc = vm_ioapic_pincount(ctx, pincount);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_isa_assert_irq(struct vmctx *ctx, int atpic_irq, int ioapic_irq)
{
	struct govmm_result r;

	r.rc = vm_isa_assert_irq(ctx, atpic_irq, ioapic_irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_isa_deassert_irq(struct vmctx *ctx, int atpic_irq, int ioapic_irq)
{
	struct govmm_result r;

	r.rc = vm_isa_deassert_irq(ctx, atpic_irq, ioapic_irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_isa_pulse_irq(struct vmctx *ctx, int atpic_irq, int ioapic_irq)
{
	struct govmm_result r;

	r.rc = vm_isa_pulse_irq(ctx, atpic_irq, ioapic_irq);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_isa_set_irq_trigger(struct vmctx *ctx, int atpic_irq, int trigger)
{
	struct govmm_result r;

	r.rc = vm_isa_set_irq_trigger(ctx, atpic_irq,
	    (enum vm_intr_trigger)trigger);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_intinfo(struct vcpu *vcpu, uint64_t *i1, uint64_t *i2)
{
	struct govmm_result r;

	r.rc = vm_get_intinfo(vcpu, i1, i2);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_set_intinfo(struct vcpu *vcpu, uint64_t exit_intinfo)
{
	struct govmm_result r;

	r.rc = vm_set_intinfo(vcpu, exit_intinfo);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_x2apic_state(struct vcpu *vcpu, int *state)
{
	struct govmm_result r;
	enum x2apic_state s;

	s = X2APIC_DISABLED;
	r.rc = vm_get_x2apic_state(vcpu, &s);
	r.err = errno;
	if (r.rc == 0)
		*state = (int)s;
	return (r);
}

struct govmm_result
govmm_vm_set_x2apic_state(struct vcpu *vcpu, int state)
{
	struct govmm_result r;

	r.rc = vm_set_x2apic_state(vcpu, (enum x2apic_state)state);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_hpet_capabilities(struct vmctx *ctx, uint32_t *capabilities)
{
	struct govmm_result r;

	r.rc = vm_get_hpet_capabilities(ctx, capabilities);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_apicid2vcpu(struct vmctx *ctx, int apicid)
{
	struct govmm_result r;

	r.rc = vm_apicid2vcpu(ctx, apicid);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_readwrite_kernemu_device(struct vcpu *vcpu, uint64_t gpa, int write,
    int size, uint64_t *value)
{
	struct govmm_result r;

	r.rc = vm_readwrite_kernemu_device(vcpu, gpa, write != 0, size, value);
	r.err = errno;
	return (r);
}

/* PCI passthrough ------------------------------------------------------- */

struct govmm_result
govmm_vm_assign_pptdev(struct vmctx *ctx, int bus, int slot, int func)
{
	struct govmm_result r;

	r.rc = vm_assign_pptdev(ctx, bus, slot, func);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_unassign_pptdev(struct vmctx *ctx, int bus, int slot, int func)
{
	struct govmm_result r;

	r.rc = vm_unassign_pptdev(ctx, bus, slot, func);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_map_pptdev_mmio(struct vmctx *ctx, int bus, int slot, int func,
    uint64_t gpa, size_t len, uint64_t hpa)
{
	struct govmm_result r;

	r.rc = vm_map_pptdev_mmio(ctx, bus, slot, func, gpa, len, hpa);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_unmap_pptdev_mmio(struct vmctx *ctx, int bus, int slot, int func,
    uint64_t gpa, size_t len)
{
	struct govmm_result r;

	r.rc = vm_unmap_pptdev_mmio(ctx, bus, slot, func, gpa, len);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_setup_pptdev_msi(struct vmctx *ctx, int bus, int slot, int func,
    uint64_t addr, uint64_t msg, int numvec)
{
	struct govmm_result r;

	r.rc = vm_setup_pptdev_msi(ctx, bus, slot, func, addr, msg, numvec);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_setup_pptdev_msix(struct vmctx *ctx, int bus, int slot, int func,
    int idx, uint64_t addr, uint64_t msg, uint32_t vector_control)
{
	struct govmm_result r;

	r.rc = vm_setup_pptdev_msix(ctx, bus, slot, func, idx, addr, msg,
	    vector_control);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_disable_pptdev_msix(struct vmctx *ctx, int bus, int slot, int func)
{
	struct govmm_result r;

	r.rc = vm_disable_pptdev_msix(ctx, bus, slot, func);
	r.err = errno;
	return (r);
}

/* RTC, amd64 ------------------------------------------------------------ */

struct govmm_result
govmm_vm_rtc_read(struct vmctx *ctx, int offset, uint8_t *value)
{
	struct govmm_result r;

	r.rc = vm_rtc_read(ctx, offset, value);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_rtc_write(struct vmctx *ctx, int offset, uint8_t value)
{
	struct govmm_result r;

	r.rc = vm_rtc_write(ctx, offset, value);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_rtc_gettime(struct vmctx *ctx, int64_t *secs)
{
	struct govmm_result r;
	time_t t;

	t = 0;
	r.rc = vm_rtc_gettime(ctx, &t);
	r.err = errno;
	if (r.rc == 0)
		*secs = (int64_t)t;
	return (r);
}

struct govmm_result
govmm_vm_rtc_settime(struct vmctx *ctx, int64_t secs)
{
	struct govmm_result r;

	r.rc = vm_rtc_settime(ctx, (time_t)secs);
	r.err = errno;
	return (r);
}

/* Address translation --------------------------------------------------- */

/*
 * Fill in a kernel paging descriptor from the flat one Go passes.
 */
static void
govmm_paging_copyin(struct vm_guest_paging *out,
    const struct govmm_paging *in)
{

	memset(out, 0, sizeof(*out));
	out->cr3 = in->cr3;
	out->cpl = in->cpl;
	out->cpu_mode = (enum vm_cpu_mode)in->cpu_mode;
	out->paging_mode = (enum vm_paging_mode)in->paging_mode;
}

struct govmm_result
govmm_vm_gla2gpa(struct vcpu *vcpu, const struct govmm_paging *paging,
    uint64_t gla, int prot, uint64_t *gpa, int *fault)
{
	struct govmm_result r;
	struct vm_guest_paging pg;

	govmm_paging_copyin(&pg, paging);
	*fault = 0;
	r.rc = vm_gla2gpa(vcpu, &pg, gla, prot, gpa, fault);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_gla2gpa_nofault(struct vcpu *vcpu, const struct govmm_paging *paging,
    uint64_t gla, int prot, uint64_t *gpa, int *fault)
{
	struct govmm_result r;
	struct vm_guest_paging pg;

	govmm_paging_copyin(&pg, paging);
	*fault = 0;
	r.rc = vm_gla2gpa_nofault(vcpu, &pg, gla, prot, gpa, fault);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_get_gpa_pmap(struct vmctx *ctx, uint64_t gpa, uint64_t *pte, int *num)
{
	struct govmm_result r;

	r.rc = vm_get_gpa_pmap(ctx, gpa, pte, num);
	r.err = errno;
	return (r);
}

/* Guest virtual memory access ------------------------------------------- */

/*
 * One chunk of a guest virtual copy, in pages.
 *
 * vm_copy_setup() asserts that it has an iovec left for every page it
 * touches, so a short array would abort the process rather than fail.
 * The copy is therefore done in bounded chunks, with two spare entries
 * to cover a partial page at each end.
 */
#define	GOVMM_COPY_PAGES	64
#define	GOVMM_COPY_IOV_MAX	(GOVMM_COPY_PAGES + 2)

/*
 * Translate and copy one chunk in the requested direction.
 */
static int
govmm_copy_chunk(struct vcpu *vcpu, struct vm_guest_paging *pg, uint64_t gla,
    void *host, size_t len, bool write, int *fault)
{
	struct iovec iov[GOVMM_COPY_IOV_MAX];
	int error, prot;

	prot = write ? PROT_WRITE : PROT_READ;

	error = vm_copy_setup(vcpu, pg, gla, len, prot, iov,
	    GOVMM_COPY_IOV_MAX, fault);
	if (error != 0 || *fault != 0)
		return (error);

	if (write)
		vm_copyout(host, iov, len);
	else
		vm_copyin(iov, host, len);

	vm_copy_teardown(iov, GOVMM_COPY_IOV_MAX);
	return (0);
}

/*
 * Walk a guest virtual range in chunks, copying in the given direction.
 */
static struct govmm_result
govmm_copy_guest_virtual(struct vcpu *vcpu, const struct govmm_paging *paging,
    uint64_t gla, void *host, size_t len, bool write, int *fault)
{
	struct govmm_result r;
	struct vm_guest_paging pg;
	char *p;
	size_t chunk, max;

	r.rc = 0;
	r.err = 0;
	*fault = 0;

	govmm_paging_copyin(&pg, paging);
	p = host;

	/*
	 * A chunk may start part way into a page, so cap it one page
	 * short of what the iovec array can describe.
	 */
	max = (size_t)(GOVMM_COPY_PAGES - 1) * PAGE_SIZE;

	while (len > 0) {
		chunk = len < max ? len : max;

		r.rc = govmm_copy_chunk(vcpu, &pg, gla, p, chunk, write,
		    fault);
		r.err = errno;
		if (r.rc != 0 || *fault != 0)
			return (r);

		gla += chunk;
		p += chunk;
		len -= chunk;
	}

	r.err = 0;
	return (r);
}

struct govmm_result
govmm_vm_read_guest_virtual(struct vcpu *vcpu,
    const struct govmm_paging *paging, uint64_t gla, void *dst, size_t len,
    int *fault)
{

	return (govmm_copy_guest_virtual(vcpu, paging, gla, dst, len, false,
	    fault));
}

struct govmm_result
govmm_vm_write_guest_virtual(struct vcpu *vcpu,
    const struct govmm_paging *paging, uint64_t gla, const void *src,
    size_t len, int *fault)
{

	/*
	 * vm_copyout() takes a const source; the cast drops the const
	 * only to reach the shared chunk helper.
	 */
	return (govmm_copy_guest_virtual(vcpu, paging, gla,
	    __DECONST(void *, src), len, true, fault));
}

/* Snapshots and Capsicum ------------------------------------------------ */

struct govmm_result
govmm_vm_snapshot_req(struct vmctx *ctx, const char *dev_name, int dev_req,
    int op, void *buf, size_t buflen, size_t *used)
{
	struct govmm_result r;
	struct vm_snapshot_meta meta;

	memset(&meta, 0, sizeof(meta));
	meta.dev_name = dev_name;
	meta.dev_req = (enum snapshot_req)dev_req;
	meta.op = (enum vm_snapshot_op)op;

	/*
	 * buf_start and buf_size are const in the ABI, so the whole
	 * buffer descriptor is built by assignment through a local copy
	 * rather than field by field.
	 */
	{
		struct vm_snapshot_buffer b = {
			.buf_start = buf,
			.buf_size = buflen,
			.buf = buf,
			.buf_rem = buflen,
		};

		memcpy(&meta.buffer, &b, sizeof(b));
	}

	r.rc = vm_snapshot_req(ctx, &meta);
	r.err = errno;

	*used = buflen - meta.buffer.buf_rem;
	return (r);
}

struct govmm_result
govmm_vm_restore_time(struct vmctx *ctx)
{
	struct govmm_result r;

	r.rc = vm_restore_time(ctx);
	r.err = errno;
	return (r);
}

struct govmm_result
govmm_vm_limit_rights(struct vmctx *ctx)
{
	struct govmm_result r;

	r.rc = vm_limit_rights(ctx);
	r.err = errno;
	return (r);
}
