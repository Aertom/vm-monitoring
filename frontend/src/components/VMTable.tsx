import React, { useState } from 'react';
import { VM, Family } from '../types';
import './VMTable.css';

interface VMTableProps {
  vms: VM[];
  families: Family[];
  loading: boolean;
}

export const VMTable: React.FC<VMTableProps> = ({
  vms,
  families,
  loading,
}) => {
  const [selectedFamily, setSelectedFamily] = useState<Family | 'all'>('all');

  const filteredVMs = selectedFamily === 'all'
    ? vms
    : vms.filter((vm) => vm.family === selectedFamily);

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
      ) : filteredVMs.length === 0 ? (
        <p>No VMs available</p>
      ) : (
        <table className="vm-table">
          <thead>
            <tr>
              <th>Hostname</th>
              <th>IP Address</th>
              <th>Family</th>
              <th>Status</th>
              <th>In Use By</th>
              <th>Checked Out At</th>
            </tr>
          </thead>
          <tbody>
            {filteredVMs.map((vm) => (
              <tr key={vm.id} className={`status-${vm.status}`}>
                <td>{vm.hostname}</td>
                <td>{vm.ip}</td>
                <td>
                  <span className={`family-badge family-${vm.family}`}>
                    {vm.family}
                  </span>
                </td>
                <td>{vm.status}</td>
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
