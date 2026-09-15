import React, { useState } from 'react';
import { Group } from '../types';
import { apiService } from '../api/client';
import { VmCells } from './VmCells';
import './GroupTable.css';

interface GroupTableProps {
  groups: Group[];
  onGroupsUpdated: () => void;
  loading: boolean;
}

const groupTitle = (group: Group): string => group.name || group.id;

export const GroupTable: React.FC<GroupTableProps> = ({
  groups,
  onGroupsUpdated,
  loading,
}) => {
  const [actingGroupId, setActingGroupId] = useState<string | null>(null);
  const [userInputs, setUserInputs] = useState<Record<string, string>>({});
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null);
  const [nameInputs, setNameInputs] = useState<Record<string, string>>({});
  const [sortMode, setSortMode] = useState<'name' | 'available-first' | 'checkedout-first'>('name');

  const sortedGroups = [...groups].sort((a, b) => {
    if (sortMode !== 'name') {
      const rank = (g: Group) => (g.status === 'available' ? 0 : 1);
      const order = sortMode === 'available-first' ? 1 : -1;
      const diff = (rank(a) - rank(b)) * order;
      if (diff !== 0) return diff;
    }
    return groupTitle(a).localeCompare(groupTitle(b));
  });

  const handleCheckout = async (groupId: string) => {
    const user = (userInputs[groupId] || '').trim();
    if (!user) {
      alert('Please enter a user name');
      return;
    }
    setActingGroupId(groupId);
    try {
      await apiService.checkoutGroup(groupId, user);
      setUserInputs((prev) => ({ ...prev, [groupId]: '' }));
      onGroupsUpdated();
    } catch (error) {
      alert(`Checkout failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setActingGroupId(null);
    }
  };

  const handleCheckin = async (groupId: string) => {
    setActingGroupId(groupId);
    try {
      await apiService.checkinGroup(groupId);
      onGroupsUpdated();
    } catch (error) {
      alert(`Checkin failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setActingGroupId(null);
    }
  };

  const handleRename = async (groupId: string) => {
    const name = (nameInputs[groupId] || '').trim();
    if (!name) {
      alert('Please enter a group name');
      return;
    }
    setActingGroupId(groupId);
    try {
      await apiService.renameGroup(groupId, name);
      setEditingGroupId(null);
      onGroupsUpdated();
    } catch (error) {
      alert(`Rename failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setActingGroupId(null);
    }
  };

  if (loading) {
    return (
      <div className="group-table-container">
        <h2>Groups</h2>
        <p>Loading groups...</p>
      </div>
    );
  }

  if (sortedGroups.length === 0) {
    return (
      <div className="group-table-container">
        <h2>Groups</h2>
        <p>No groups available</p>
      </div>
    );
  }

  return (
    <div className="group-list">
      <div className="group-list-header">
        <h2>Groups</h2>
        <div className="family-filter">
          <label htmlFor="group-sort-select">Sort by:</label>
          <select
            id="group-sort-select"
            value={sortMode}
            onChange={(e) => setSortMode(e.target.value as typeof sortMode)}
          >
            <option value="name">Name</option>
            <option value="available-first">Available first</option>
            <option value="checkedout-first">Checked out first</option>
          </select>
        </div>
      </div>
      {sortedGroups.map((group) => (
        <section key={group.id} aria-label={`Group ${groupTitle(group)}`} className="group-table-container group-card">
          <div className="group-card-header">
            <div className="group-card-title">
              {editingGroupId === group.id ? (
                <div className="checkout-input-group">
                  <input
                    type="text"
                    aria-label="Group name"
                    placeholder="Group name"
                    maxLength={64}
                    value={nameInputs[group.id] ?? group.name ?? ''}
                    onChange={(e) =>
                      setNameInputs((prev) => ({ ...prev, [group.id]: e.target.value }))
                    }
                    disabled={actingGroupId !== null}
                  />
                  <button
                    onClick={() => handleRename(group.id)}
                    disabled={actingGroupId !== null}
                    className="btn-checkout"
                  >
                    Save
                  </button>
                  <button
                    onClick={() => setEditingGroupId(null)}
                    disabled={actingGroupId !== null}
                    className="btn-checkin"
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <>
                  <h3>
                    <strong className="group-name">{group.name || group.id}</strong>
                    {group.name && (
                      <span className="group-id" title={group.id}>
                        {' '}{group.id}
                      </span>
                    )}
                    <button
                      aria-label={`Rename group ${groupTitle(group)}`}
                      title="Rename group"
                      onClick={() => {
                        setNameInputs((prev) => ({ ...prev, [group.id]: group.name ?? '' }));
                        setEditingGroupId(group.id);
                      }}
                      disabled={actingGroupId !== null}
                      className="btn-rename"
                    >
                      ✎
                    </button>
                  </h3>
                  <div className="group-card-meta">
                    <span className={`pill pill-${group.status}`}>{group.status}</span>
                    <span>In Use By: <strong>{group.inUseBy || '-'}</strong></span>
                    <span>
                      Checked Out At:{' '}
                      {group.checkedOutAt ? new Date(group.checkedOutAt).toLocaleString() : '-'}
                    </span>
                    <span>{group.vms?.length ?? 0} VMs</span>
                  </div>
                </>
              )}
            </div>
            <div className="group-card-actions">
              {group.status === 'available' ? (
                <div className="checkout-input-group">
                  <input
                    type="text"
                    placeholder="User"
                    aria-label={`Checkout user for ${groupTitle(group)}`}
                    value={userInputs[group.id] || ''}
                    onChange={(e) =>
                      setUserInputs((prev) => ({ ...prev, [group.id]: e.target.value }))
                    }
                    disabled={actingGroupId !== null}
                  />
                  <button
                    onClick={() => handleCheckout(group.id)}
                    disabled={actingGroupId !== null}
                    className="btn-checkout"
                  >
                    {actingGroupId === group.id ? 'Checking...' : 'Checkout'}
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => handleCheckin(group.id)}
                  disabled={actingGroupId !== null}
                  className="btn-checkin"
                >
                  {actingGroupId === group.id ? 'Checking in...' : 'Checkin'}
                </button>
              )}
            </div>
          </div>
          <div className="table-scroll">
            <table className="group-table">
              <thead>
                <tr>
                  <th>Hostname</th>
                  <th>IP Address</th>
                  <th>Hypervisor</th>
                  <th>Family</th>
                  <th>Status</th>
                  <th>Version</th>
                  <th>In Use By</th>
                  <th>SSH</th>
                </tr>
              </thead>
              <tbody>
                {(group.vms ?? []).map((vm) => (
                  <tr key={vm.id} className={`status-${vm.status}`}>
                    <VmCells vm={vm} showGroup={false} />
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ))}
    </div>
  );
};
