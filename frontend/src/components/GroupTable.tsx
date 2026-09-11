import React, { useState } from 'react';
import { Group } from '../types';
import { apiService } from '../api/client';
import './GroupTable.css';

interface GroupTableProps {
  groups: Group[];
  onGroupsUpdated: () => void;
  loading: boolean;
}

type GroupSortKey = 'id' | 'vms' | 'status' | 'inUseBy' | 'checkedOutAt';

const groupVMsLabel = (group: Group): string =>
  group.vms?.length
    ? group.vms.map((vm) => vm.hostname).join(', ')
    : Object.entries(group.members || {})
        .map(([fam, id]) => `${fam}:${id}`)
        .join(', ') || 'N/A';

const groupSortValue = (group: Group, key: GroupSortKey): string => {
  switch (key) {
    case 'id':
      return group.name || group.id;
    case 'vms':
      return groupVMsLabel(group);
    case 'status':
      return group.status;
    case 'inUseBy':
      return group.inUseBy ?? '';
    case 'checkedOutAt':
      return group.checkedOutAt ?? '';
  }
};

export const GroupTable: React.FC<GroupTableProps> = ({
  groups,
  onGroupsUpdated,
  loading,
}) => {
  const [actingGroupId, setActingGroupId] = useState<string | null>(null);
  const [userInputs, setUserInputs] = useState<Record<string, string>>({});
  const [editingGroupId, setEditingGroupId] = useState<string | null>(null);
  const [nameInputs, setNameInputs] = useState<Record<string, string>>({});
  const [sortKey, setSortKey] = useState<GroupSortKey | null>(null);
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

  const toggleSort = (key: GroupSortKey) => {
    if (sortKey !== key) {
      setSortKey(key);
      setSortDir('asc');
    } else {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    }
  };

  const arrow = (key: GroupSortKey) => (sortKey === key ? (sortDir === 'asc' ? ' ▲' : ' ▼') : '');

  const th = (label: string, key: GroupSortKey) => (
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

  const sortedGroups = sortKey
    ? [...groups].sort((a, b) => {
        const cmp = groupSortValue(a, sortKey).localeCompare(groupSortValue(b, sortKey));
        return sortDir === 'asc' ? cmp : -cmp;
      })
    : groups;

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

  return (
    <div className="group-table-container">
      <h2>Groups</h2>
      {loading ? (
        <p>Loading groups...</p>
      ) : sortedGroups.length === 0 ? (
        <p>No groups available</p>
      ) : (
        <div className="table-scroll">
        <table className="group-table">
          <thead>
            <tr>
              {th('Group ID', 'id')}
              {th('VMs', 'vms')}
              {th('Status', 'status')}
              {th('In Use By', 'inUseBy')}
              {th('Checked Out At', 'checkedOutAt')}
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {sortedGroups.map((group) => (
              <tr key={group.id} className={`status-${group.status}`}>
                <td>
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
                      <strong className="group-name">{group.name || group.id}</strong>
                      {group.name && <div className="group-id" title={group.id}>{group.id}</div>}
                      <button
                        aria-label={`Rename group ${group.name || group.id}`}
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
                    </>
                  )}
                </td>
                <td>{groupVMsLabel(group)}</td>
                <td><span className={`pill pill-${group.status}`}>{group.status}</span></td>
                <td>{group.inUseBy || '-'}</td>
                <td>
                  {group.checkedOutAt
                    ? new Date(group.checkedOutAt).toLocaleString()
                    : '-'}
                </td>
                <td className="action-cell">
                  {group.status === 'available' ? (
                    <div className="checkout-input-group">
                      <input
                        type="text"
                        placeholder="User"
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
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      )}
    </div>
  );
};
