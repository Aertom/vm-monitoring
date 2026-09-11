import { VM, Group, enrichVMsWithCheckout } from './index';

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

describe('enrichVMsWithCheckout', () => {
  const vm: VM = {
    id: 'sm1',
    hostname: 'sm-prod-01',
    ip: '10.0.0.1',
    family: 'sm',
    status: 'ok',
  };

  it('propage le checkout du groupe vers ses VMs', () => {
    const groups: Group[] = [
      {
        id: 'g1',
        members: { sm: 'sm1' },
        vms: [vm],
        status: 'checkedOut',
        inUseBy: 'alice',
        checkedOutAt: '2026-09-11T08:00:00Z',
      },
    ];
    const [enriched] = enrichVMsWithCheckout([vm], groups);
    expect(enriched.inUseBy).toBe('alice');
    expect(enriched.checkedOutAt).toBe('2026-09-11T08:00:00Z');
    expect(enriched.groupId).toBe('g1');
  });

  it('retrouve le groupe via members quand vms est vide', () => {
    const groups: Group[] = [
      { id: 'g1', members: { sm: 'sm1' }, vms: [], status: 'checkedOut', inUseBy: 'bob' },
    ];
    const [enriched] = enrichVMsWithCheckout([vm], groups);
    expect(enriched.inUseBy).toBe('bob');
    expect(enriched.groupId).toBe('g1');
  });

  it('laisse la VM intacte sans groupe et renseigne groupId si libre', () => {
    expect(enrichVMsWithCheckout([vm], [])[0]).toEqual(vm);
    const groups: Group[] = [
      { id: 'g1', members: { sm: 'sm1' }, vms: [vm], status: 'available' },
    ];
    const [enriched] = enrichVMsWithCheckout([vm], groups);
    expect(enriched.groupId).toBe('g1');
    expect(enriched.inUseBy).toBeUndefined();
  });
});
