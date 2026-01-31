// Auth store for managing current agent identity and authentication state.

import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';

// Agent represents the current user's agent identity.
export interface Agent {
  id: number;
  name: string;
  createdAt: string;
  lastActiveAt: string;
}

// AgentStatus represents the real-time status of an agent.
export type AgentStatusType = 'active' | 'busy' | 'idle' | 'offline';

export interface AgentStatus {
  agentId: number;
  status: AgentStatusType;
  sessionId?: number;
  lastHeartbeat: string;
}

interface AuthState {
  // Current agent (the "logged in" identity).
  currentAgent: Agent | null;
  currentAgentStatus: AgentStatus | null;

  // Authentication state.
  isAuthenticated: boolean;
  isLoading: boolean;

  // Available agents for switching.
  availableAgents: Agent[];

  // Actions.
  setCurrentAgent: (agent: Agent | null) => void;
  setCurrentAgentStatus: (status: AgentStatus | null) => void;
  setAvailableAgents: (agents: Agent[]) => void;
  setLoading: (loading: boolean) => void;
  switchAgent: (agentId: number) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  devtools(
    persist(
      (set, get) => ({
        // Initial state.
        currentAgent: null,
        currentAgentStatus: null,
        isAuthenticated: false,
        isLoading: true,
        availableAgents: [],

        // Set the current agent and mark as authenticated.
        setCurrentAgent: (agent) =>
          set(
            {
              currentAgent: agent,
              isAuthenticated: agent !== null,
              isLoading: false,
            },
            undefined,
            'setCurrentAgent',
          ),

        // Update the current agent's status.
        setCurrentAgentStatus: (status) =>
          set({ currentAgentStatus: status }, undefined, 'setCurrentAgentStatus'),

        // Set the list of available agents.
        setAvailableAgents: (agents) =>
          set({ availableAgents: agents }, undefined, 'setAvailableAgents'),

        // Set loading state.
        setLoading: (loading) =>
          set({ isLoading: loading }, undefined, 'setLoading'),

        // Switch to a different agent by ID.
        switchAgent: (agentId) => {
          const state = get();
          const agent = state.availableAgents.find((a) => a.id === agentId);
          if (agent) {
            set(
              {
                currentAgent: agent,
                isAuthenticated: true,
                currentAgentStatus: null,
              },
              undefined,
              'switchAgent',
            );
          }
        },

        // Log out and clear current agent.
        logout: () =>
          set(
            {
              currentAgent: null,
              currentAgentStatus: null,
              isAuthenticated: false,
            },
            undefined,
            'logout',
          ),
      }),
      {
        name: 'auth-storage',
        partialize: (state) => ({
          currentAgent: state.currentAgent,
        }),
      },
    ),
    { name: 'auth-store' },
  ),
);
