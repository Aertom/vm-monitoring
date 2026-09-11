import axios from 'axios';

jest.mock('axios');
const mockedAxios = axios as jest.Mocked<typeof axios>;
const mockGet = jest.fn();
const mockPost = jest.fn();
(mockedAxios.create as unknown as jest.Mock).mockReturnValue({ get: mockGet, post: mockPost });

const { apiService } = require('./client');

describe('apiService', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (mockedAxios.create as unknown as jest.Mock).mockReturnValue({ get: mockGet, post: mockPost });
  });

  it('getVMs retourne les données', async () => {
    const vms = [{ id: 'vm1' }];
    mockGet.mockResolvedValue({ data: vms });
    await expect(apiService.getVMs()).resolves.toEqual(vms);
    expect(mockGet).toHaveBeenCalledWith('/vms', { params: {} });
  });

  it('getVMs avec filtre famille', async () => {
    mockGet.mockResolvedValue({ data: [] });
    await apiService.getVMs('sm');
    expect(mockGet).toHaveBeenCalledWith('/vms', { params: { family: 'sm' } });
  });

  it('getVMs retourne [] si data nulle et propage les erreurs', async () => {
    mockGet.mockResolvedValue({ data: null });
    await expect(apiService.getVMs()).resolves.toEqual([]);
    mockGet.mockRejectedValueOnce(new Error('net'));
    await expect(apiService.getVMs()).rejects.toThrow('net');
  });

  it('getGroups / getFamilies', async () => {
    mockGet.mockResolvedValue({ data: [{ id: 'g1' }] });
    await expect(apiService.getGroups()).resolves.toEqual([{ id: 'g1' }]);
    mockGet.mockResolvedValue({ data: ['sm'] });
    await expect(apiService.getFamilies()).resolves.toEqual(['sm']);
  });

  it('checkout envoie user + inUseBy (compat backend)', async () => {
    const group = { id: 'g1' };
    mockPost.mockResolvedValue({ data: group });
    await expect(apiService.checkoutGroup('g1', 'alice')).resolves.toEqual(group);
    expect(mockPost).toHaveBeenCalledWith('/groups/g1/checkout', { user: 'alice', inUseBy: 'alice' });
  });

  it('checkin', async () => {
    const group = { id: 'g1' };
    mockPost.mockResolvedValue({ data: group });
    await expect(apiService.checkinGroup('g1')).resolves.toEqual(group);
    expect(mockPost).toHaveBeenCalledWith('/groups/g1/checkin', {});
  });

  it('same-origin appelle une URL relative', async () => {
    const prev = process.env.REACT_APP_API_URL;
    process.env.REACT_APP_API_URL = 'same-origin';
    jest.resetModules();
    try {
      const freshAxios = require('axios');
      freshAxios.create.mockReturnValue({ get: mockGet, post: mockPost });
      const fresh = require('./client').apiService;
      expect(freshAxios.create).toHaveBeenCalledWith(
        expect.objectContaining({ baseURL: '/api' })
      );
      mockGet.mockResolvedValue({ data: [] });
      await expect(fresh.getVMs()).resolves.toEqual([]);
    } finally {
      if (prev === undefined) delete process.env.REACT_APP_API_URL;
      else process.env.REACT_APP_API_URL = prev;
      jest.resetModules();
    }
  });

  it('rename', async () => {
    const group = { id: 'g1', name: 'prod' };
    mockPost.mockResolvedValue({ data: group });
    await expect(apiService.renameGroup('g1', 'prod')).resolves.toEqual(group);
    expect(mockPost).toHaveBeenCalledWith('/groups/g1/rename', { name: 'prod' });
  });
});
