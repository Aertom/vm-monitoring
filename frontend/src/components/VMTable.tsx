import React, { useState } from 'react';
import { VM, Family } from '../types';
import './VMTable.css';

interface VMTableProps {
  vms: VM[];
  families: Family[];
  loading: boolean;
}

type VMSortKey = 'hostname' | 'ip' | 'family' | 'status' | 'groupId' | 'inUseBy' | 'checkedOutAt';

export const VMTable: React.FC<VMTableProps> = ({
  vms,
  families,
  loading,
}) => {
  const [selectedFamily, setSelectedFamily] = useState<Family | 'all'>('all');
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

  const filteredVMs = selectedFamily === 'all'
    ? vms
    : vms.filter((vm) => vm.family === selectedFamily);

  const sortedVMs = sortKey
    ? [...filteredVMs].sort((a, b) => {
        const cmp = String(a[sortKey] ?? '').localeCompare(String(b[sortKey] ?? ''));
        return sortDir === 'asc' ? cmp : -cmp;
      })
    : filteredVMs;

  return (
    <div className="vm-table-container">
      <div className="vm-header">
        <h2>Virtual Machines</h2>
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
      </div>

      {loading ? (
        <p>Loading VMs...</p>
      ) : sortedVMs.length === 0 ? (
        <p>No VMs available</p>
      ) : (
        <table className="vm-table">
          <thead>
            <tr>
              {th('Hostname', 'hostname')}
              {th('IP Address', 'ip')}
              {th('Family', 'family')}
              {th('Status', 'status')}
              {th('Group', 'groupId')}
              {th('In Use By', 'inUseBy')}
              {th('Checked Out At', 'checkedOutAt')}
            </tr>
          </thead>
          <tbody>
            {sortedVMs.map((vm) => (
              <tr key={vm.id} className={`status-${vm.status}`}>
                <td>{vm.hostname}</td>
                <td>{vm.ip}</td>
                <td>
                  <span className={`family-badge family-${vm.family}`}>
                    {vm.family}
                  </span>
                </td>
                <td>{vm.status}</td>
                <td title={vm.groupId || ''}>{vm.groupId ? vm.groupId.slice(0, 8) : '-'}</td>
                <td>{vm.inUseBy || '-'}</td>
                <td>
                  {vm.checkedOutAt
                    ? new Date(vm.checkedOutAt).toLocaleString()
                    : '-'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
};
