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
  inUseBy?: string;
  checkedOutAt?: string;
}

export interface Group {
  id: string;
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
