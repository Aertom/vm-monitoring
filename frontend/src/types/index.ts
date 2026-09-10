export type Family = 'sm' | 'cm' | 'ws' | 'oa' | 'unknown';

export type Status = 'available' | 'checkedOut' | 'error';

export interface VM {
  id: string;
  hostname: string;
  ip: string;
  family: Family;
  status: Status;
  inUseBy?: string;
  checkedOutAt?: string;
}

export interface Group {
  id: string;
  vms: VM[];
  status: Status;
  inUseBy?: string;
  checkedOutAt?: string;
}

export interface ApiResponse<T> {
  data?: T;
  error?: string;
}
