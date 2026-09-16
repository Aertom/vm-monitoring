import React, { useEffect, useState } from 'react';
import { Family } from '../types';
import { apiService, CreationOptions, HypervisorRef } from '../api/client';
import './CreatePage.css';

const errMsg = (error: unknown, fallback: string): string => {
  const e = error as { response?: { data?: { error?: string } }; message?: string };
  return e?.response?.data?.error || e?.message || fallback;
};

export const CreatePage: React.FC = () => {
  const [hypervisors, setHypervisors] = useState<HypervisorRef[]>([]);
  const [families, setFamilies] = useState<Family[]>([]);
  const [options, setOptions] = useState<CreationOptions | null>(null);

  const [hv, setHv] = useState('');
  const [family, setFamily] = useState<Family | ''>('');
  const [name, setName] = useState('');
  const [vmType, setVmType] = useState('');
  const [isoFile, setIsoFile] = useState('');
  const [datastore, setDatastore] = useState('');
  const [network, setNetwork] = useState('');
  const [cpu, setCpu] = useState(2);
  const [ramGB, setRamGB] = useState(8);
  const [diskGB, setDiskGB] = useState(60);
  const [ip, setIp] = useState('');
  const [ipMsg, setIpMsg] = useState<string | null>(null);
  const [dryRun, setDryRun] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiService.listHypervisors().then(setHypervisors).catch(() => setHypervisors([]));
    apiService.getFamilies().then(setFamilies).catch(() => setFamilies([]));
  }, []);

  const applyType = (t: string, opts: CreationOptions | null) => {
    setVmType(t);
    const preset = opts?.types?.[t];
    if (preset) {
      setCpu(preset.cpu);
      setRamGB(preset.ramGB);
      setDiskGB(preset.diskGB);
    }
  };

  const handleHv = async (value: string) => {
    setHv(value);
    setResult(null);
    setError(null);
    if (!value) {
      setOptions(null);
      return;
    }
    try {
      const opts = await apiService.creationOptions(value);
      setOptions(opts);
      setDatastore(opts.datastore || '');
      setNetwork(opts.network || '');
      const firstISO = opts.isos?.[0]?.file || '';
      setIsoFile(firstISO);
      const types = Object.keys(opts.types || {});
      applyType(types[0] || '', opts);
    } catch (e) {
      setError(errMsg(e, 'Options indisponibles'));
      setOptions(null);
    }
  };

  const handleDetect = async () => {
    if (!name.trim()) return;
    try {
      setFamily(await apiService.detectFamily(name.trim()));
    } catch (e) {
      setError(errMsg(e, 'Détection impossible'));
    }
  };

  const handleSuggestIP = async () => {
    if (!hv) return;
    try {
      setIp(await apiService.suggestIP(hv));
      setIpMsg(null);
    } catch (e) {
      setIpMsg(errMsg(e, 'Suggestion impossible'));
    }
  };

  const handleCheckIP = async () => {
    if (!hv || !ip.trim()) {
      setIpMsg(null);
      return;
    }
    try {
      const r = await apiService.checkIP(hv, ip.trim());
      if (r.used) setIpMsg('IP déjà utilisée');
      else if (!r.inRange) setIpMsg('IP hors du sous-réseau');
      else setIpMsg(null);
    } catch {
      setIpMsg(null);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setResult(null);
    setError(null);
    try {
      const r = await apiService.createVM(
        {
          hypervisor: hv, family, name: name.trim(), type: vmType, isoFile,
          datastore, network, cpu, ramGB, diskGB, ip: ip.trim(),
        },
        dryRun
      );
      setResult(
        dryRun
          ? `DRY-RUN — commandes générées :\n${(r.commands || []).join('\n---\n')}`
          : `VM ${r.name} créée sur ${r.hypervisor} (${r.ip}), power on OK.\n${r.log || ''}`
      );
    } catch (err) {
      setError(errMsg(err, 'Création échouée'));
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = hv && family && name.trim() && vmType && isoFile && ip.trim() && !submitting;

  return (
    <div className="create-container">
      <h2>Create VM</h2>
      <form onSubmit={handleSubmit} className="create-form">
        <div className="form-row">
          <label>
            Hypervisor
            <select value={hv} onChange={(e) => handleHv(e.target.value)} required>
              <option value="">— Choisir —</option>
              {hypervisors.map((h) => (
                <option key={h.name} value={h.name}>
                  {h.name} ({h.type})
                </option>
              ))}
            </select>
          </label>
          <label>
            Family
            <select value={family} onChange={(e) => setFamily(e.target.value as Family)} required>
              <option value="">— Choisir —</option>
              {families.map((f) => (
                <option key={f} value={f}>
                  {f}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="form-row">
          <label className="grow">
            VM name (libre)
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={handleDetect}
              placeholder="ex: cm-prod-09"
              required
            />
          </label>
          <button type="button" onClick={handleDetect} disabled={!name.trim()} className="btn-checkin">
            Detect family
          </button>
        </div>

        <div className="form-row">
          <label>
            Type
            <select value={vmType} onChange={(e) => applyType(e.target.value, options)} required>
              <option value="">— Choisir —</option>
              {Object.keys(options?.types || {}).map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </label>
          <label>
            ISO RedHat
            <select value={isoFile} onChange={(e) => setIsoFile(e.target.value)} required>
              <option value="">— Choisir —</option>
              {(options?.isos || []).map((iso) => (
                <option key={iso.file} value={iso.file}>
                  {iso.name}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="form-row">
          <label>
            CPU
            <input type="number" min={1} max={64} value={cpu} onChange={(e) => setCpu(Number(e.target.value))} required />
          </label>
          <label>
            RAM (Go)
            <input type="number" min={1} max={1024} value={ramGB} onChange={(e) => setRamGB(Number(e.target.value))} required />
          </label>
          <label>
            Disque (Go)
            <input type="number" min={10} max={4000} value={diskGB} onChange={(e) => setDiskGB(Number(e.target.value))} required />
          </label>
        </div>

        <div className="form-row">
          <label>
            Datastore / pool
            <input type="text" value={datastore} onChange={(e) => setDatastore(e.target.value)} required />
          </label>
          <label>
            Network
            <input type="text" value={network} onChange={(e) => setNetwork(e.target.value)} required />
          </label>
        </div>

        <div className="form-row">
          <label className="grow">
            IP statique
            <input
              type="text"
              value={ip}
              onChange={(e) => setIp(e.target.value)}
              onBlur={handleCheckIP}
              placeholder="ex: 192.168.1.50"
              required
            />
          </label>
          <button type="button" onClick={handleSuggestIP} disabled={!hv} className="btn-checkin">
            Suggest IP
          </button>
        </div>
        {ipMsg && <p className="field-error">{ipMsg}</p>}

        <div className="form-row">
          <label className="unknown-toggle">
            <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
            Dry run (commandes sans exécution)
          </label>
          <button type="submit" disabled={!canSubmit} className="btn-checkout">
            {submitting ? 'Creating...' : dryRun ? 'Preview' : 'Create VM'}
          </button>
        </div>
      </form>

      {error && (
        <div className="error-banner">
          <strong>Error:</strong> {error}
        </div>
      )}
      {result && <pre className="result-panel">{result}</pre>}
    </div>
  );
};
