// Unit tests for the auth store.

import { describe, it, expect, beforeEach } from 'vitest';
import { useAuthStore, type Agent } from '@/stores/auth';

describe('useAuthStore', () => {
  const mockAgent: Agent = {
    id: 1,
    name: 'TestAgent',
    createdAt: '2025-01-01T00:00:00Z',
    lastActiveAt: '2025-01-31T12:00:00Z',
  };

  const mockAgent2: Agent = {
    id: 2,
    name: 'AnotherAgent',
    createdAt: '2025-01-02T00:00:00Z',
    lastActiveAt: '2025-01-31T11:00:00Z',
  };

  beforeEach(() => {
    // Reset the store to initial state before each test.
    useAuthStore.setState({
      currentAgent: null,
      currentAgentStatus: null,
      selectedAgentIds: [],
      selectedAggregate: null,
      isGlobalExplicit: false,
      isAuthenticated: false,
      isLoading: true,
      availableAgents: [],
    });
  });

  describe('setCurrentAgent', () => {
    it('should set current agent and mark as authenticated', () => {
      const store = useAuthStore.getState();
      store.setCurrentAgent(mockAgent);

      const state = useAuthStore.getState();
      expect(state.currentAgent).toEqual(mockAgent);
      expect(state.isAuthenticated).toBe(true);
      expect(state.isLoading).toBe(false);
    });

    it('should clear authentication when set to null', () => {
      const store = useAuthStore.getState();
      store.setCurrentAgent(mockAgent);
      store.setCurrentAgent(null);

      const state = useAuthStore.getState();
      expect(state.currentAgent).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe('setCurrentAgentStatus', () => {
    it('should update agent status', () => {
      const store = useAuthStore.getState();
      const status = {
        agentId: 1,
        status: 'active' as const,
        lastHeartbeat: '2025-01-31T12:00:00Z',
      };
      store.setCurrentAgentStatus(status);

      expect(useAuthStore.getState().currentAgentStatus).toEqual(status);
    });

    it('should accept status with session ID', () => {
      const store = useAuthStore.getState();
      const status = {
        agentId: 1,
        status: 'busy' as const,
        sessionId: 42,
        lastHeartbeat: '2025-01-31T12:00:00Z',
      };
      store.setCurrentAgentStatus(status);

      const state = useAuthStore.getState();
      expect(state.currentAgentStatus?.status).toBe('busy');
      expect(state.currentAgentStatus?.sessionId).toBe(42);
    });
  });

  describe('setAvailableAgents', () => {
    it('should set the list of available agents', () => {
      const store = useAuthStore.getState();
      store.setAvailableAgents([mockAgent, mockAgent2]);

      expect(useAuthStore.getState().availableAgents).toHaveLength(2);
    });
  });

  describe('switchAgent', () => {
    it('should switch to a different agent by ID', () => {
      const store = useAuthStore.getState();
      store.setAvailableAgents([mockAgent, mockAgent2]);
      store.setCurrentAgent(mockAgent);
      store.switchAgent(2);

      const state = useAuthStore.getState();
      expect(state.currentAgent?.id).toBe(2);
      expect(state.currentAgent?.name).toBe('AnotherAgent');
      expect(state.isAuthenticated).toBe(true);
    });

    it('should clear agent status when switching', () => {
      const store = useAuthStore.getState();
      store.setAvailableAgents([mockAgent, mockAgent2]);
      store.setCurrentAgent(mockAgent);
      store.setCurrentAgentStatus({
        agentId: 1,
        status: 'active',
        lastHeartbeat: '2025-01-31T12:00:00Z',
      });
      store.switchAgent(2);

      expect(useAuthStore.getState().currentAgentStatus).toBeNull();
    });

    it('should not change state for non-existent agent ID', () => {
      const store = useAuthStore.getState();
      store.setAvailableAgents([mockAgent, mockAgent2]);
      store.setCurrentAgent(mockAgent);
      store.switchAgent(999);

      expect(useAuthStore.getState().currentAgent?.id).toBe(1);
    });
  });

  describe('logout', () => {
    it('should clear current agent and authentication state', () => {
      const store = useAuthStore.getState();
      store.setCurrentAgent(mockAgent);
      store.setCurrentAgentStatus({
        agentId: 1,
        status: 'active',
        lastHeartbeat: '2025-01-31T12:00:00Z',
      });
      store.logout();

      const state = useAuthStore.getState();
      expect(state.currentAgent).toBeNull();
      expect(state.currentAgentStatus).toBeNull();
      expect(state.isAuthenticated).toBe(false);
    });
  });

  describe('setLoading', () => {
    it('should update loading state', () => {
      const store = useAuthStore.getState();
      store.setLoading(false);

      expect(useAuthStore.getState().isLoading).toBe(false);

      store.setLoading(true);
      expect(useAuthStore.getState().isLoading).toBe(true);
    });
  });

  describe('selectAggregate', () => {
    it('should set aggregate selection with multiple agent IDs', () => {
      const store = useAuthStore.getState();
      store.selectAggregate({
        name: 'CodeReviewer',
        agentIds: [1, 2, 3],
      });

      const state = useAuthStore.getState();
      expect(state.selectedAggregate).toBe('CodeReviewer');
      expect(state.selectedAgentIds).toEqual([1, 2, 3]);
      expect(state.currentAgent).toBeNull();
      expect(state.isAuthenticated).toBe(true);
    });

    it('should clear isGlobalExplicit when selecting aggregate', () => {
      const store = useAuthStore.getState();
      store.clearSelection(); // Sets isGlobalExplicit to true.
      store.selectAggregate({
        name: 'CodeReviewer',
        agentIds: [1, 2],
      });

      expect(useAuthStore.getState().isGlobalExplicit).toBe(false);
    });
  });

  describe('clearSelection', () => {
    it('should clear selection and set isGlobalExplicit', () => {
      const store = useAuthStore.getState();
      store.setCurrentAgent(mockAgent);
      store.clearSelection();

      const state = useAuthStore.getState();
      expect(state.currentAgent).toBeNull();
      expect(state.selectedAgentIds).toEqual([]);
      expect(state.selectedAggregate).toBeNull();
      expect(state.isGlobalExplicit).toBe(true);
    });
  });

  describe('setAvailableAgents with User default', () => {
    const userAgent: Agent = {
      id: 999,
      name: 'User',
      createdAt: '2025-01-01T00:00:00Z',
      lastActiveAt: '2025-01-31T12:00:00Z',
    };

    it('should auto-select User agent on fresh load', () => {
      const store = useAuthStore.getState();
      store.setAvailableAgents([mockAgent, userAgent]);

      const state = useAuthStore.getState();
      expect(state.currentAgent?.name).toBe('User');
      expect(state.selectedAgentIds).toEqual([999]);
      expect(state.isAuthenticated).toBe(true);
    });

    it('should not auto-select User if agent already selected', () => {
      const store = useAuthStore.getState();
      store.setCurrentAgent(mockAgent);
      store.setAvailableAgents([mockAgent, userAgent]);

      const state = useAuthStore.getState();
      expect(state.currentAgent?.name).toBe('TestAgent');
    });

    it('should not auto-select User if isGlobalExplicit is true', () => {
      const store = useAuthStore.getState();
      store.clearSelection(); // Sets isGlobalExplicit to true.
      store.setAvailableAgents([mockAgent, userAgent]);

      const state = useAuthStore.getState();
      expect(state.currentAgent).toBeNull();
      expect(state.isGlobalExplicit).toBe(true);
    });
  });
});
