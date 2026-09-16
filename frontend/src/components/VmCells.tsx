import React, { useEffect, useRef, useState } from 'react';
import { VM } from '../types';

// Copie presse-papiers avec repli (clipboard API absente ou refusée).
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // repli ci-dessous
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

export const sshCommand = (vm: VM): string => `ssh -XAC admin@${vm.ip}`;

// Bouton SSH : copie `ssh -XAC admin@ip` dans le presse-papiers,
// à coller dans un terminal.
export const SshButton: React.FC<{ vm: VM }> = ({ vm }) => {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  useEffect(() => () => {
    if (timer.current !== null) window.clearTimeout(timer.current);
  }, []);

  if (!vm.ip) return <span>-</span>;
  const cmd = sshCommand(vm);
  const handleClick = async () => {
    if (await copyText(cmd)) {
      setCopied(true);
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), 1500);
    } else {
      alert(`Copie impossible, commande : ${cmd}`);
    }
  };

  return (
    <button
      onClick={handleClick}
      title={cmd}
      aria-label={`Copy SSH command for ${vm.hostname}`}
      className="btn-ssh"
    >
      {copied ? 'Copied' : 'SSH'}
    </button>
  );
};

const versionLabel = (vm: VM): string =>
  (vm.apps ?? []).map((a) => (a.version ? `${a.name} ${a.version}` : a.name)).join(', ');

// Colonnes des tableaux VMs (ordre d'affichage, labels pour show/hide).
export const VM_COLUMNS = [
  { key: 'hostname', label: 'Hostname', sortable: true },
  { key: 'ip', label: 'IP Address', sortable: true },
  { key: 'hypervisor', label: 'Hypervisor', sortable: true },
  { key: 'os', label: 'OS', sortable: true },
  { key: 'family', label: 'Family', sortable: true },
  { key: 'status', label: 'Status', sortable: true },
  { key: 'groupId', label: 'Group', sortable: true },
  { key: 'version', label: 'Version', sortable: true },
  { key: 'inUseBy', label: 'In Use By', sortable: true },
  { key: 'ssh', label: 'SSH', sortable: false },
] as const;

export type VMColumnKey = (typeof VM_COLUMNS)[number]['key'];

export type VMColumnVisibility = Record<VMColumnKey, boolean>;

export const allVisible = (): VMColumnVisibility => ({
  hostname: true,
  ip: true,
  hypervisor: true,
  os: true,
  family: true,
  status: true,
  groupId: true,
  version: true,
  inUseBy: true,
  ssh: true,
});

// Cellules communes aux tableaux VMs et groupes (mêmes infos partout),
// filtrées par visibilité (show/hide sur la page VMs).
export const VmCells: React.FC<{ vm: VM; visible?: VMColumnVisibility }> = ({
  vm,
  visible,
}) => {
  const v = visible ?? allVisible();
  return (
    <>
      {v.hostname && <td>{vm.hostname}</td>}
      {v.ip && <td>{vm.ip}</td>}
      {v.hypervisor && (
        <td title={vm.hypervisorName ? `type: ${vm.hypervisor || '?'}` : ''}>
          {vm.hypervisorName || vm.hypervisor ? (
            <span className="id-chip">{vm.hypervisorName || vm.hypervisor}</span>
          ) : (
            '-'
          )}
        </td>
      )}
      {v.os && <td>{vm.os || '-'}</td>}
      {v.family && (
        <td>
          <span className={`family-badge family-${vm.family}`}>{vm.family}</span>
        </td>
      )}
      {v.status && (
        <td>
          <span
            className={`pill pill-${vm.status}`}
            title={vm.status === 'error' && vm.lastError ? vm.lastError : undefined}
          >
            {vm.status}
          </span>
        </td>
      )}
      {v.groupId && (
        <td title={vm.groupId || ''}>
          {vm.groupName ? (
            <strong>{vm.groupName}</strong>
          ) : vm.groupId ? (
            <span className="id-chip">{vm.groupId.slice(0, 8)}</span>
          ) : (
            '-'
          )}
        </td>
      )}
      {v.version && (
        <td title={versionLabel(vm)}>
          {vm.apps?.length ? (
            vm.apps.map((app, i) => (
              <div key={i} className="app-version">
                {app.name}
                {app.version ? <span className="app-version-nb"> {app.version}</span> : null}
              </div>
            ))
          ) : (
            '-'
          )}
        </td>
      )}
      {v.inUseBy && <td>{vm.inUseBy || '-'}</td>}
      {v.ssh && (
        <td>
          <SshButton vm={vm} />
        </td>
      )}
    </>
  );
};
