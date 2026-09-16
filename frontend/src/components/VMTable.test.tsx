import { render, screen, fireEvent, within, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { VMTable } from './VMTable';
import { VM } from '../types';

const vms: VM[] = [
  {
    id: 'sm1',
    hostname: 'sm-prod-01',
    ip: '10.0.0.1',
    family: 'sm',
    status: 'ok',
    os: 'Ubuntu 22.04.5 LTS',
    apps: [
      { name: 'appli1', version: '1.2.3' },
      { name: 'appli2', version: '2.0' },
    ],
  },
  { id: 'cm1', hostname: 'cm-prod-01', ip: '10.0.0.2', family: 'cm', status: 'error' },
];

describe('VMTable', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });
  it('affiche les VMs et filtre par famille', () => {
    render(<VMTable vms={vms} families={['sm', 'cm']} loading={false} />);
    expect(screen.getByText('sm-prod-01')).toBeInTheDocument();
    expect(screen.getByText('cm-prod-01')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Filter by Family/i), { target: { value: 'sm' } });
    expect(screen.getByText('sm-prod-01')).toBeInTheDocument();
    expect(screen.queryByText('cm-prod-01')).not.toBeInTheDocument();
  });

  it('états loading et vide', () => {
    const { rerender } = render(<VMTable vms={[]} families={[]} loading={true} />);
    expect(screen.getByText(/Loading VMs/i)).toBeInTheDocument();
    rerender(<VMTable vms={[]} families={[]} loading={false} />);
    expect(screen.getByText(/No VMs available/i)).toBeInTheDocument();
  });

  it('trie par colonne au clic sur les en-têtes', () => {
    render(<VMTable vms={vms} families={['sm', 'cm']} loading={false} />);
    const table = screen.getByRole('table');
    const firstRow = () => within(table).getAllByRole('row')[1].textContent;
    expect(firstRow()).toContain('sm-prod-01');
    fireEvent.click(screen.getByRole('columnheader', { name: /hostname/i }));
    expect(firstRow()).toContain('cm-prod-01');
    fireEvent.click(screen.getByRole('columnheader', { name: /hostname/i }));
    expect(firstRow()).toContain('sm-prod-01');
  });

  it('affiche les versions des applications', () => {
    render(<VMTable vms={vms} families={['sm', 'cm']} loading={false} />);
    expect(screen.getByText('appli1')).toBeInTheDocument();
    expect(screen.getByText('1.2.3')).toBeInTheDocument();
    expect(screen.queryByText(/Checked Out At/i)).not.toBeInTheDocument();
  });

  it('affiche le nom hyperviseur (découverte) sinon le type', () => {
    const discovered: VM[] = [
      { id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok', hypervisor: 'esxi', hypervisorName: 'esx-08' },
      { id: 'cm1', hostname: 'cm-prod-01', ip: '10.0.0.2', family: 'cm', status: 'ok', hypervisor: 'static' },
      { id: 'x1', hostname: 'x-prod-01', ip: '10.0.0.9', family: 'unknown', status: 'unknown' },
    ];
    render(<VMTable vms={discovered} families={['sm', 'cm', 'unknown']} loading={false} />);
    const table = screen.getByRole('table');
    expect(within(table).getByText('esx-08')).toBeInTheDocument();
    expect(within(table).getByText('static')).toBeInTheDocument();
  });

  it('accepte une famille configurée (wks) sans classe dédiée', () => {
    const custom: VM[] = [
      { id: 'w1', hostname: 'wks-prod-01', ip: '10.0.0.1', family: 'wks', status: 'ok' },
    ];
    render(<VMTable vms={custom} families={['wks']} loading={false} />);
    expect(screen.getByText('wks-prod-01')).toBeInTheDocument();
    expect(screen.getByText('wks')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Filter by Family/i), { target: { value: 'wks' } });
    expect(screen.getByText('wks-prod-01')).toBeInTheDocument();
  });

  it('affiche la cause au survol du statut error', () => {
    const failed: VM[] = [
      { id: 'cm1', hostname: 'cm-prod-01', ip: '10.0.0.2', family: 'cm', status: 'error', lastError: 'connexion ssh 10.0.0.2: timeout' },
    ];
    render(<VMTable vms={failed} families={['cm']} loading={false} />);
    expect(screen.getByTitle('connexion ssh 10.0.0.2: timeout')).toHaveTextContent('error');
  });

  it('affiche la colonne OS et masque les colonnes décochées', () => {
    render(<VMTable vms={vms} families={['sm', 'cm']} loading={false} />);
    expect(screen.getByText('Ubuntu 22.04.5 LTS')).toBeInTheDocument();
    const panel = screen.getByText('Columns').closest('details') as HTMLElement;
    const osBox = within(panel).getByLabelText('OS') as HTMLInputElement;
    fireEvent.click(osBox);
    expect(screen.queryByText('Ubuntu 22.04.5 LTS')).not.toBeInTheDocument();
    expect(screen.getByText('sm-prod-01')).toBeInTheDocument();
  });

  it('cache unknown par défaut, filtre par hyperviseur', () => {
    const mixed: VM[] = [
      { id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok', hypervisor: 'esxi', hypervisorName: 'esx-08' },
      { id: 'x1', hostname: 'x-prod-01', ip: '10.0.0.9', family: 'unknown', status: 'unknown', hypervisor: 'static' },
    ];
    render(<VMTable vms={mixed} families={['sm']} loading={false} />);
    expect(screen.queryByText('x-prod-01')).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText(/Show unknown/i));
    expect(screen.getByText('x-prod-01')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Filter by Hypervisor/i), { target: { value: 'esx-08' } });
    expect(screen.getByText('sm-prod-01')).toBeInTheDocument();
    expect(screen.queryByText('x-prod-01')).not.toBeInTheDocument();
  });

  it('affiche le nom du groupe et copie la commande SSH', async () => {
    const withGroup: VM[] = [
      { id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok', groupId: 'g1', groupName: 'prod' },
    ];
    const writeText = jest.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<VMTable vms={withGroup} families={['sm']} loading={false} />);
    expect(screen.getByText('prod')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Copy SSH command/i }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('ssh -XAC admin@10.0.0.1'));
    expect(await screen.findByText('Copied')).toBeInTheDocument();
  });
});
