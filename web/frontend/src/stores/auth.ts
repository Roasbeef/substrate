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

// Aggregate agent group (e.g., "CodeReviewer" for all reviewer-* agents).
export interface AgentAggregate {
  name: string;
  agentIds: number[];
}

interface AuthState {
  // Current agent (the "logged in" identity).
  currentAgent: Agent | null;
  currentAgentStatus: AgentStatus | null;

  // For aggregate selections (e.g., "CodeReviewer" shows all reviewer-* agents).
  // When set, inbox filters by all these agent IDs.
  selectedAgentIds: number[];

  // Current aggregate name if an aggregate is selected (null for single agent).
  selectedAggregate: string | null;

  // Tracks whether user explicitly selected Global view (to distinguish from fresh load).
  isGlobalExplicit: boolean;

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
  selectAggregate: (aggregate: AgentAggregate) => void;
  clearSelection: () => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  devtools(
    persist(
      (set, get) => ({
        // Initial state.
        currentAgent: null,
        currentAgentStatus: null,
        selectedAgentIds: [],
        selectedAggregate: null,
        isGlobalExplicit: false,
        isAuthenticated: false,
        isLoading: true,
        availableAgents: [],

        // Set the current agent and mark as authenticated.
        // Clears isGlobalExplicit since user is selecting a specific agent.
        setCurrentAgent: (agent) =>
          set(
            {
              currentAgent: agent,
              selectedAgentIds: agent ? [agent.id] : [],
              selectedAggregate: null,
              isGlobalExplicit: false,
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
        // If no agent is currently selected (fresh load), default to "User" agent.
        // Skip auto-selection if user explicitly chose Global view previously.
        setAvailableAgents: (agents) => {
          const state = get();
          // Only auto-select User if:
          // 1. No agent is selected
          // 2. No aggregate is selected
          // 3. User didn't explicitly choose Global view
          if (
            state.currentAgent === null &&
            state.selectedAggregate === null &&
            !state.isGlobalExplicit
          ) {
            const userAgent = agents.find((a) => a.name === 'User');
            if (userAgent) {
              set(
                {
                  availableAgents: agents,
                  currentAgent: userAgent,
                  selectedAgentIds: [userAgent.id],
                  selectedAggregate: null,
                  isAuthenticated: true,
                  isLoading: false,
                },
                undefined,
                'setAvailableAgents:defaultToUser',
              );
              return;
            }
          }
          set({ availableAgents: agents }, undefined, 'setAvailableAgents');
        },

        // Set loading state.
        setLoading: (loading) =>
          set({ isLoading: loading }, undefined, 'setLoading'),

        // Switch to a different agent by ID.
        // Clears isGlobalExplicit since user is selecting a specific agent.
        switchAgent: (agentId) => {
          const state = get();
          const agent = state.availableAgents.find((a) => a.id === agentId);
          if (agent) {
            set(
              {
                currentAgent: agent,
                selectedAgentIds: [agentId],
                selectedAggregate: null,
                isGlobalExplicit: false,
                isAuthenticated: true,
                currentAgentStatus: null,
              },
              undefined,
              'switchAgent',
            );
          }
        },

        // Select an aggregate (e.g., "CodeReviewer" for all reviewer-* agents).
        // Clears isGlobalExplicit since user is selecting an aggregate.
        selectAggregate: (aggregate) =>
          set(
            {
              currentAgent: null,
              selectedAgentIds: aggregate.agentIds,
              selectedAggregate: aggregate.name,
              isGlobalExplicit: false,
              isAuthenticated: true,
              currentAgentStatus: null,
            },
            undefined,
            'selectAggregate',
          ),

        // Clear selection (go to Global view).
        // Sets isGlobalExplicit to remember user explicitly chose Global.
        clearSelection: () =>
          set(
            {
              currentAgent: null,
              selectedAgentIds: [],
              selectedAggregate: null,
              isGlobalExplicit: true,
            },
            undefined,
            'clearSelection',
          ),

        // Log out and clear current agent.
        logout: () =>
          set(
            {
              currentAgent: null,
              currentAgentStatus: null,
              selectedAgentIds: [],
              selectedAggregate: null,
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
          isGlobalExplicit: state.isGlobalExplicit,
        }),
      },
    ),
    { name: 'auth-store' },
  ),
);
