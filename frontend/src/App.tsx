import React, { useEffect, useMemo, useState } from 'react';
import { VM, Group, Family, enrichVMsWithCheckout } from './types';
import { apiService } from './api/client';
import { VMTable } from './components/VMTable';
import { GroupTable } from './components/GroupTable';
import { CreatePage } from './components/CreatePage';
import './App.css';

const App: React.FC = () => {
  const [vms, setVMs] = useState<VM[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [families, setFamilies] = useState<Family[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState<'vms' | 'groups' | 'create'>('vms');

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

        <nav className="page-tabs" aria-label="Pages">
          <button
            className={page === 'vms' ? 'tab active' : 'tab'}
            aria-current={page === 'vms' ? 'page' : undefined}
            onClick={() => setPage('vms')}
          >
            Virtual Machines
          </button>
          <button
            className={page === 'groups' ? 'tab active' : 'tab'}
            aria-current={page === 'groups' ? 'page' : undefined}
            onClick={() => setPage('groups')}
          >
            Groups
          </button>
          <button
            className={page === 'create' ? 'tab active' : 'tab'}
            aria-current={page === 'create' ? 'page' : undefined}
            onClick={() => setPage('create')}
          >
            Create
          </button>
        </nav>

        {page === 'vms' ? (
          <VMTable vms={enrichedVMs} families={families} loading={loading} />
        ) : page === 'groups' ? (
          <GroupTable
            groups={groups}
            onGroupsUpdated={handleGroupsUpdated}
            loading={loading}
          />
        ) : (
          <CreatePage />
        )}
      </main>

      <footer className="app-footer">
        <p>
          API Base URL: {process.env.REACT_APP_API_URL || 'http://localhost:8080'}
          {' | '}
          <a
            href={`${process.env.REACT_APP_API_URL === 'same-origin' ? '' : process.env.REACT_APP_API_URL || 'http://localhost:8080'}/api/docs`}
            target="_blank"
            rel="noreferrer"
          >
            API docs
          </a>
        </p>
      </footer>
    </div>
  );
};

export default App;
