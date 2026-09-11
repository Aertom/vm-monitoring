import { VM, Group } from './index';

describe('types smoke', () => {
  it('creates VM and Group objects', () => {
    const vm: VM = {
      id: 'vm-sm-01',
      hostname: 'sm-prod-01',
      ip: '192.168.1.10',
      family: 'sm',
      status: 'ok',
    };
    const group: Group = {
      id: 'group-1',
      members: { sm: vm.id },
      vms: [vm],
      status: 'available',
    };
    expect(group.vms).toHaveLength(1);
    expect(vm.family).toBe('sm');
  });

  it('supports members fallback without vms detail', () => {
    const group: Group = {
      id: 'group-2',
      members: { sm: 'vm-sm-01', cm: 'vm-cm-01' },
      vms: [],
      status: 'checkedOut',
      inUseBy: 'alice',
    };
    expect(group.members.sm).toBe('vm-sm-01');
    expect(group.status).toBe('checkedOut');
  });
});
