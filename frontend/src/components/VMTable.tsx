import React, { useState } from 'react';
import { VM, Family } from '../types';
import { VmCells } from './VmCells';
import './VMTable.css';

interface VMTableProps {
  vms: VM[];
  families: Family[];
  loading: boolean;
}

type VMSortKey = 'hostname' | 'ip' | 'hypervisor' | 'family' | 'status' | 'groupId' | 'version' | 'inUseBy';

const versionLabel = (vm: VM): string =>
  (vm.apps ?? []).map((a) => (a.version ? `${a.name} ${a.version}` : a.name)).join(', ');

const hypervisorLabel = (vm: VM): string => vm.hypervisorName || vm.hypervisor || '';

const groupLabel = (vm: VM): string => vm.groupName || vm.groupId || '';

const vmSortValue = (vm: VM, key: VMSortKey): string => {
  if (key === 'version') return versionLabel(vm);
  if (key === 'hypervisor') return hypervisorLabel(vm);
  if (key === 'groupId') return groupLabel(vm);
  return vm[key] ?? '';
};

export const VMTable: React.FC<VMTableProps> = ({
  vms,
  families,
  loading,
}) => {
  const [selectedFamily, setSelectedFamily] = useState<Family | 'all'>('all');
  const [selectedHypervisor, setSelectedHypervisor] = useState<string>('all');
  const [showUnknown, setShowUnknown] = useState(false);
  const [sortKey, setSortKey] = useState<VMSortKey | null>(null);
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

  const toggleSort = (key: VMSortKey) => {
    if (sortKey !== key) {
      setSortKey(key);
      setSortDir('asc');
    } else {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    }
  };

  const arrow = (key: VMSortKey) => (sortKey === key ? (sortDir === 'asc' ? ' ▲' : ' ▼') : '');

  const th = (label: string, key: VMSortKey) => (
    <th
      onClick={() => toggleSort(key)}
      style={{ cursor: 'pointer', userSelect: 'none' }}
      aria-sort={sortKey !== key ? 'none' : sortDir === 'asc' ? 'ascending' : 'descending'}
      title={`Sort by ${label}`}
    >
      {label}
      <span aria-hidden="true">{arrow(key)}</span>
    </th>
  );

  const hypervisors = Array.from(new Set(vms.map(hypervisorLabel).filter((h) => h !== ''))).sort();

  const filteredVMs = vms.filter((vm) => {
    if (!showUnknown && vm.family === 'unknown') return false;
    if (selectedFamily !== 'all' && vm.family !== selectedFamily) return false;
    if (selectedHypervisor !== 'all' && hypervisorLabel(vm) !== selectedHypervisor) return false;
    return true;
  });

  const sortedVMs = sortKey
    ? [...filteredVMs].sort((a, b) => {
        const cmp = vmSortValue(a, sortKey).localeCompare(vmSortValue(b, sortKey));
        return sortDir === 'asc' ? cmp : -cmp;
      })
    : filteredVMs;

  return (
    <div className="vm-table-container">
      <div className="vm-header">
        <h2>Virtual Machines</h2>
        <div className="vm-filters">
          <div className="family-filter">
            <label htmlFor="family-select">Filter by Family:</label>
            <select
              id="family-select"
              value={selectedFamily}
              onChange={(e) => setSelectedFamily(e.target.value as Family | 'all')}
            >
              <option value="all">All</option>
              {families.map((family) => (
                <option key={family} value={family}>
                  {family.toUpperCase()}
                </option>
              ))}
            </select>
          </div>
          <div className="family-filter">
            <label htmlFor="hypervisor-select">Filter by Hypervisor:</label>
            <select
              id="hypervisor-select"
              value={selectedHypervisor}
              onChange={(e) => setSelectedHypervisor(e.target.value)}
            >
              <option value="all">All</option>
              {hypervisors.map((h) => (
                <option key={h} value={h}>
                  {h}
                </option>
              ))}
            </select>
          </div>
          <label className="unknown-toggle">
            <input
              type="checkbox"
              checked={showUnknown}
              onChange={(e) => setShowUnknown(e.target.checked)}
            />
            Show unknown
          </label>
        </div>
      </div>

      {loading ? (
        <p>Loading VMs...</p>
      ) : sortedVMs.length === 0 ? (
        <p>No VMs available</p>
      ) : (
        <div className="table-scroll">
        <table className="vm-table">
          <thead>
            <tr>
              {th('Hostname', 'hostname')}
              {th('IP Address', 'ip')}
              {th('Hypervisor', 'hypervisor')}
              {th('Family', 'family')}
              {th('Status', 'status')}
              {th('Group', 'groupId')}
              {th('Version', 'version')}
              {th('In Use By', 'inUseBy')}
              <th>SSH</th>
            </tr>
          </thead>
          <tbody>
            {sortedVMs.map((vm) => (
              <tr key={vm.id} className={`status-${vm.status}`}>
                <VmCells vm={vm} showGroup />
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      )}
    </div>
  );
};
