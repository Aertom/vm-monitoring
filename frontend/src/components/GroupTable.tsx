import React, { useState } from 'react';
import { Group } from '../types';
import { apiService } from '../api/client';
import './GroupTable.css';

interface GroupTableProps {
  groups: Group[];
  onGroupsUpdated: () => void;
  loading: boolean;
}

export const GroupTable: React.FC<GroupTableProps> = ({
  groups,
  onGroupsUpdated,
  loading,
}) => {
  const [actingGroupId, setActingGroupId] = useState<string | null>(null);
  const [checkoutUser, setCheckoutUser] = useState<string>('');

  const handleCheckout = async (groupId: string) => {
    if (!checkoutUser.trim()) {
      alert('Please enter a user name');
      return;
    }
    setActingGroupId(groupId);
    try {
      await apiService.checkoutGroup(groupId, checkoutUser);
      setCheckoutUser('');
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

  return (
    <div className="group-table-container">
      <h2>Groups</h2>
      {loading ? (
        <p>Loading groups...</p>
      ) : groups.length === 0 ? (
        <p>No groups available</p>
      ) : (
        <table className="group-table">
          <thead>
            <tr>
              <th>Group ID</th>
              <th>VMs</th>
              <th>Status</th>
              <th>In Use By</th>
              <th>Checked Out At</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {groups.map((group) => (
              <tr key={group.id} className={`status-${group.status}`}>
                <td>{group.id}</td>
                <td>
                  {group.vms.map((vm) => vm.hostname).join(', ') || 'N/A'}
                </td>
                <td>{group.status}</td>
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
                        value={checkoutUser}
                        onChange={(e) => setCheckoutUser(e.target.value)}
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
      )}
    </div>
  );
};
