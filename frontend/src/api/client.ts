import axios, { AxiosInstance } from 'axios';
import { VM, Group, Family } from '../types';

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
};
