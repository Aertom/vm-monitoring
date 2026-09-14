import { render, screen, fireEvent } from '@testing-library/react';
import '@testing-library/jest-dom';
import App from './App';
import { apiService } from './api/client';
import { VM, Group } from './types';

jest.mock('./api/client', () => ({
  apiService: {
    getVMs: jest.fn(),
    getGroups: jest.fn(),
    getFamilies: jest.fn(),
    checkoutGroup: jest.fn(),
    checkinGroup: jest.fn(),
    renameGroup: jest.fn(),
  },
}));
const mocked = apiService as jest.Mocked<typeof apiService>;

const vms: VM[] = [
  { id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok' },
];

const groups: Group[] = [
  {
    id: 'g1',
    members: { sm: 'sm1' },
    vms,
    status: 'checkedOut',
    inUseBy: 'alice',
    checkedOutAt: '2026-09-11T08:00:00Z',
  },
];

describe('App', () => {
  beforeEach(() => {
    mocked.getVMs.mockResolvedValue(vms);
    mocked.getGroups.mockResolvedValue(groups);
    mocked.getFamilies.mockResolvedValue(['sm']);
  });

  it('page VMs : checkout propagé, puis page Groups', async () => {
    render(<App />);
    expect(await screen.findAllByText('sm-prod-01')).toHaveLength(1);
    expect(await screen.findAllByText('alice')).toHaveLength(1);

    fireEvent.click(screen.getByRole('button', { name: 'Groups' }));
    expect(await screen.findAllByText('sm-prod-01')).toHaveLength(1);
    expect(await screen.findAllByText('alice')).toHaveLength(1);
    expect(screen.getByText('g1')).toBeInTheDocument();
  });
});
