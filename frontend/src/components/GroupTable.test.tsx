import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { GroupTable } from './GroupTable';
import { apiService } from '../api/client';
import { Group } from '../types';

jest.mock('../api/client', () => ({
  apiService: { checkoutGroup: jest.fn(), checkinGroup: jest.fn() },
}));
const mocked = apiService as jest.Mocked<typeof apiService>;

const groups: Group[] = [
  {
    id: 'g1',
    members: { sm: 'sm1' },
    vms: [{ id: 'sm1', hostname: 'sm-prod-01', ip: '10.0.0.1', family: 'sm', status: 'ok' }],
    status: 'available',
  },
  {
    id: 'g2',
    members: { sm: 'sm2' },
    vms: [{ id: 'sm2', hostname: 'sm-prod-02', ip: '10.0.0.3', family: 'sm', status: 'ok' }],
    status: 'checkedOut',
    inUseBy: 'alice',
  },
];

describe('GroupTable', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.alert = jest.fn();
  });

  it('affiche les groupes avec fallback members', () => {
    render(<GroupTable groups={groups} onGroupsUpdated={jest.fn()} loading={false} />);
    expect(screen.getByText('g1')).toBeInTheDocument();
    expect(screen.getByText('sm-prod-01')).toBeInTheDocument();
    expect(screen.getByText('alice')).toBeInTheDocument();
  });

  it('alerte si checkout sans user, puis checkout par ligne', async () => {
    const onUpdated = jest.fn();
    (mocked.checkoutGroup as jest.Mock).mockResolvedValue({});
    render(<GroupTable groups={groups} onGroupsUpdated={onUpdated} loading={false} />);
    const buttons = screen.getAllByText('Checkout');
    fireEvent.click(buttons[0]);
    expect(window.alert).toHaveBeenCalled();
    const inputs = screen.getAllByPlaceholderText('User');
    fireEvent.change(inputs[0], { target: { value: 'bob' } });
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(mocked.checkoutGroup).toHaveBeenCalledWith('g1', 'bob'));
    expect(onUpdated).toHaveBeenCalled();
  });

  it('checkin', async () => {
    const onUpdated = jest.fn();
    (mocked.checkinGroup as jest.Mock).mockResolvedValue({});
    render(<GroupTable groups={groups} onGroupsUpdated={onUpdated} loading={false} />);
    fireEvent.click(screen.getByText('Checkin'));
    await waitFor(() => expect(mocked.checkinGroup).toHaveBeenCalledWith('g2'));
    expect(onUpdated).toHaveBeenCalled();
  });

  it('états loading et vide', () => {
    const { rerender } = render(<GroupTable groups={[]} onGroupsUpdated={jest.fn()} loading={true} />);
    expect(screen.getByText(/Loading groups/i)).toBeInTheDocument();
    rerender(<GroupTable groups={[]} onGroupsUpdated={jest.fn()} loading={false} />);
    expect(screen.getByText(/No groups available/i)).toBeInTheDocument();
  });
});
