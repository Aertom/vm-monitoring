import { render, screen, fireEvent, within } from '@testing-library/react';
import '@testing-library/jest-dom';
import { VMTable } from './VMTable';
import { VM } from '../types';

const vms: VM[] = [
  { id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok' },
  { id: 'cm1', hostname: 'cm-prod-01', ip: '10.0.0.2', family: 'cm', status: 'error' },
];

describe('VMTable', () => {
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
});
