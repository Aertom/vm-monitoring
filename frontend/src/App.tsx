import React, { useEffect, useMemo, useState } from 'react';
import { VM, Group, Family, enrichVMsWithCheckout } from './types';
import { apiService } from './api/client';
import { VMTable } from './components/VMTable';
import { GroupTable } from './components/GroupTable';
import './App.css';

const App: React.FC = () => {
  const [vms, setVMs] = useState<VM[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [families, setFamilies] = useState<Family[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [vmsData, groupsData, familiesData] = await Promise.all([
        apiService.getVMs(),
        apiService.getGroups(),
        apiService.getFamilies(),
      ]);
      setVMs(vmsData);
      setGroups(groupsData);
      setFamilies(familiesData);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load data';
      setError(message);
      console.error('Data fetch error:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleGroupsUpdated = () => {
    fetchData();
  };

  const enrichedVMs = useMemo(() => enrichVMsWithCheckout(vms, groups), [vms, groups]);

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-content">
          <h1>VM Monitoring Dashboard</h1>
          <p>Manage hypervisor VM groups and checkout/checkin operations</p>
        </div>
        <button onClick={fetchData} disabled={loading} className="btn-refresh">
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </header>

      <main className="app-main">
        {error && (
          <div className="error-banner">
            <strong>Error:</strong> {error}
          </div>
        )}

        <VMTable vms={enrichedVMs} families={families} loading={loading} />
        <GroupTable
          groups={groups}
          onGroupsUpdated={handleGroupsUpdated}
          loading={loading}
        />
      </main>

      <footer className="app-footer">
        <p>
          API Base URL: {process.env.REACT_APP_API_URL || 'http://localhost:8080'}
        </p>
      </footer>
    </div>
  );
};

export default App;
