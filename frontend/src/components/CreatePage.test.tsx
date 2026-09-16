import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import { CreatePage } from './CreatePage';
import { apiService } from '../api/client';

jest.mock('../api/client', () => ({
  apiService: {
    listHypervisors: jest.fn(),
    getFamilies: jest.fn(),
    creationOptions: jest.fn(),
    detectFamily: jest.fn(),
    suggestIP: jest.fn(),
    checkIP: jest.fn(),
    createVM: jest.fn(),
  },
}));
const mocked = apiService as jest.Mocked<typeof apiService>;

const options = {
  kind: 'esxi',
  datastore: 'datastore1',
  network: 'VM Network',
  isoDir: 'iso',
  subnet: '10.9.0.0/24',
  isos: [{ name: 'RHEL 9.5', file: 'rhel-9.5.iso' }],
  types: { serveur: { cpu: 4, ramGB: 16, diskGB: 100 } },
};

describe('CreatePage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.alert = jest.fn();
    (mocked.listHypervisors as jest.Mock).mockResolvedValue([{ name: 'esx-08', type: 'esxi' }]);
    (mocked.getFamilies as jest.Mock).mockResolvedValue(['sm']);
    (mocked.creationOptions as jest.Mock).mockResolvedValue(options);
    (mocked.detectFamily as jest.Mock).mockResolvedValue('sm');
    (mocked.suggestIP as jest.Mock).mockResolvedValue('10.9.0.2');
    (mocked.createVM as jest.Mock).mockResolvedValue({
      dryRun: true, hypervisor: 'esx-08', name: 'sm-new-01', ip: '10.9.0.2',
      poweredOn: false, commands: ['vim-cmd solo/registervm'],
    });
  });

  it('pré-remplit le formulaire au choix de l hyperviseur', async () => {
    render(<CreatePage />);
    expect(await screen.findByText('esx-08 (esxi)')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Hypervisor/i), { target: { value: 'esx-08' } });
    expect(await screen.findByDisplayValue('datastore1')).toBeInTheDocument();
    expect(screen.getByDisplayValue('VM Network')).toBeInTheDocument();
    expect(screen.getByDisplayValue(4)).toBeInTheDocument();
  });

  it('détecte la famille, suggère une IP et prévisualise', async () => {
    const { container } = render(<CreatePage />);
    expect(await screen.findByText('esx-08 (esxi)')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Hypervisor/i), { target: { value: 'esx-08' } });
    await screen.findByDisplayValue('datastore1');

    fireEvent.change(screen.getByPlaceholderText(/ex: cm-prod-09/i), { target: { value: 'sm-new-01' } });
    fireEvent.click(screen.getByText('Detect family'));
    await waitFor(() => expect(mocked.detectFamily).toHaveBeenCalledWith('sm-new-01'));

    fireEvent.click(screen.getByText('Suggest IP'));
    expect(await screen.findByDisplayValue('10.9.0.2')).toBeInTheDocument();

    const form = container.querySelector('form');
    if (!form) throw new Error('formulaire introuvable');
    fireEvent.submit(form);
    await waitFor(() => expect(mocked.createVM).toHaveBeenCalled());
    expect(await screen.findByText(/DRY-RUN/)).toBeInTheDocument();
    expect(screen.getByText(/vim-cmd solo\/registervm/)).toBeInTheDocument();
  });
});
