import axios, { AxiosInstance } from 'axios';
import { VM, Group, Family } from '../types';

export interface HypervisorRef {
  name: string;
  type: string;
}

export interface ISOOption {
  name: string;
  file: string;
}

export interface CreationOptions {
  kind: string;
  datastore: string;
  network: string;
  isoDir: string;
  subnet: string;
  container?: string;
  isos: ISOOption[];
  types: Record<string, { cpu: number; ramGB: number; diskGB: number }>;
}

export interface CreateRequest {
  hypervisor: string;
  family: string;
  name: string;
  type: string;
  isoFile: string;
  datastore: string;
  network: string;
  cpu: number;
  ramGB: number;
  diskGB: number;
  ip: string;
}

export interface CreateResult {
  dryRun: boolean;
  hypervisor: string;
  name: string;
  ip: string;
  poweredOn: boolean;
  commands?: string[];
  log?: string;
}

const RAW_API_URL = process.env.REACT_APP_API_URL;
// 'same-origin' = appels relatifs (/api/...) via le nginx du compose (proxy /api).
// Utile en prod mono-origine ; en dev local garder http://localhost:8080.
const API_BASE_URL = RAW_API_URL === 'same-origin' ? '' : RAW_API_URL || 'http://localhost:8080';

const apiClient: AxiosInstance = axios.create({
  baseURL: `${API_BASE_URL}/api`,
  timeout: 10000,
});

export const apiService = {
  // Fetch all VMs, optionally filtered by family
  async getVMs(family?: Family): Promise<VM[]> {
    try {
      const params = family ? { family } : {};
      const response = await apiClient.get<VM[]>('/vms', { params });
      return response.data || [];
    } catch (error) {
      console.error('Failed to fetch VMs:', error);
      throw error;
    }
  },

  // Fetch all groups
  async getGroups(): Promise<Group[]> {
    try {
      const response = await apiClient.get<Group[]>('/groups');
      return response.data || [];
    } catch (error) {
      console.error('Failed to fetch groups:', error);
      throw error;
    }
  },

  // Fetch all families
  async getFamilies(): Promise<Family[]> {
    try {
      const response = await apiClient.get<Family[]>('/families');
      return response.data || [];
    } catch (error) {
      console.error('Failed to fetch families:', error);
      throw error;
    }
  },

  // Checkout a group
  async checkoutGroup(groupId: string, inUseBy: string): Promise<Group> {
    try {
      const response = await apiClient.post<Group>(
        `/groups/${groupId}/checkout`,
        { user: inUseBy, inUseBy }
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to checkout group ${groupId}:`, error);
      throw error;
    }
  },

  // Checkin a group
  async checkinGroup(groupId: string): Promise<Group> {
    try {
      const response = await apiClient.post<Group>(
        `/groups/${groupId}/checkin`,
        {}
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to checkin group ${groupId}:`, error);
      throw error;
    }
  },

  // Rename a group (alias libre, ID inchangé)
  async renameGroup(groupId: string, name: string): Promise<Group> {
    try {
      const response = await apiClient.post<Group>(
        `/groups/${groupId}/rename`,
        { name }
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to rename group ${groupId}:`, error);
      throw error;
    }
  },

  // Hyperviseurs configurés (noms + types, sans secrets)
  async listHypervisors(): Promise<HypervisorRef[]> {
    const response = await apiClient.get<HypervisorRef[]>('/hypervisors');
    return response.data || [];
  },

  // Déduit la famille d'un hostname (pré-remplissage)
  async detectFamily(hostname: string): Promise<Family> {
    const response = await apiClient.post<{ family: Family }>('/families/detect', { hostname });
    return response.data.family;
  },

  // Options de création pré-remplies pour un hyperviseur
  async creationOptions(hypervisor: string): Promise<CreationOptions> {
    const response = await apiClient.get<CreationOptions>('/creation/options', {
      params: { hypervisor },
    });
    return response.data;
  },

  // Première IP libre du sous-réseau
  async suggestIP(hypervisor: string): Promise<string> {
    const response = await apiClient.get<{ ip: string }>('/creation/suggest-ip', {
      params: { hypervisor },
    });
    return response.data.ip;
  },

  // Vérifie une IP (occupation + plage)
  async checkIP(hypervisor: string, ip: string): Promise<{ ip: string; used: boolean; inRange: boolean }> {
    const response = await apiClient.get('/creation/check-ip', {
      params: { hypervisor, ip },
    });
    return response.data;
  },

  // Crée une VM (dryRun = commandes sans exécution)
  async createVM(req: CreateRequest, dryRun: boolean): Promise<CreateResult> {
    const response = await apiClient.post<CreateResult>(
      `/creation${dryRun ? '?dryRun=true' : ''}`,
      req
    );
    return response.data;
  },
};
