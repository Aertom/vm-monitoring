export type Family = 'sm' | 'cm' | 'ws' | 'oa' | 'unknown';

export type VMStatus = 'ok' | 'error' | 'unknown';

export type GroupStatus = 'available' | 'checkedOut';

export type Status = VMStatus | GroupStatus | 'error';

export interface VM {
  id: string;
  hostname: string;
  ip: string;
  family: Family;
  status: Status;
  hypervisor?: string;
  groupId?: string;
  inUseBy?: string;
  checkedOutAt?: string;
}

export interface Group {
  id: string;
  name?: string;
  members: Partial<Record<Family, string>>;
  vms: VM[];
  status: GroupStatus;
  inUseBy?: string;
  checkedOutAt?: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: string;
}

// enrichVMsWithCheckout reporte l'état de checkout des groupes sur chaque VM
// (groupId, inUseBy, checkedOutAt). L'API ne renvoie pas ces infos sur
// GET /vms, sans cet enrichissement le tableau VMs ignore les checkout.
export function enrichVMsWithCheckout(vms: VM[], groups: Group[]): VM[] {
  const groupByVmId = new Map<string, Group>();
  for (const group of groups) {
    for (const vm of group.vms ?? []) {
      if (!groupByVmId.has(vm.id)) groupByVmId.set(vm.id, group);
    }
    for (const id of Object.values(group.members ?? {})) {
      if (id && !groupByVmId.has(id)) groupByVmId.set(id, group);
    }
  }
  return vms.map((vm) => {
    const group = groupByVmId.get(vm.id);
    if (!group) return vm;
    return {
      ...vm,
      groupId: group.id,
      inUseBy: group.inUseBy ?? vm.inUseBy,
      checkedOutAt: group.checkedOutAt ?? vm.checkedOutAt,
    };
  });
}
