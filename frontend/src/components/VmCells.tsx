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

// Cellules communes aux tableaux VMs et groupes (mêmes infos partout).
// showGroup=false dans les tableaux par groupe (redondant avec la carte).
export const VmCells: React.FC<{ vm: VM; showGroup?: boolean }> = ({ vm, showGroup = true }) => (
  <>
    <td>{vm.hostname}</td>
    <td>{vm.ip}</td>
    <td title={vm.hypervisorName ? `type: ${vm.hypervisor || '?'}` : ''}>
      {vm.hypervisorName || vm.hypervisor ? (
        <span className="id-chip">{vm.hypervisorName || vm.hypervisor}</span>
      ) : (
        '-'
      )}
    </td>
    <td>
      <span className={`family-badge family-${vm.family}`}>{vm.family}</span>
    </td>
    <td>
      <span
        className={`pill pill-${vm.status}`}
        title={vm.status === 'error' && vm.lastError ? vm.lastError : undefined}
      >
        {vm.status}
      </span>
    </td>
    {showGroup && (
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
    <td>{vm.inUseBy || '-'}</td>
    <td>
      <SshButton vm={vm} />
    </td>
  </>
);
