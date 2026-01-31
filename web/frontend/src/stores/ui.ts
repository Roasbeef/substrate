// UI store for managing application-wide UI state using Zustand.

import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

// Toast notification type definitions.
export type ToastVariant = 'success' | 'error' | 'warning' | 'info';

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface Toast {
  id: string;
  variant: ToastVariant;
  message: string;
  title?: string;
  duration?: number;
  action?: ToastAction;
}

// Modal type definitions for different modal dialogs.
export type ModalType =
  | 'compose'
  | 'thread'
  | 'newAgent'
  | 'startSession'
  | 'settings'
  | null;

// Modal data can vary depending on the modal type.
export interface ModalData {
  threadId?: number;
  agentId?: number;
  sessionId?: number;
  replyTo?: number;
}

// Sidebar navigation state.
export type SidebarSection = 'inbox' | 'starred' | 'snoozed' | 'sent' | 'archive' | 'agents' | 'sessions';

interface UIState {
  // Modal state.
  activeModal: ModalType;
  modalData: ModalData | null;

  // Toast notifications.
  toasts: Toast[];

  // Sidebar state.
  sidebarCollapsed: boolean;
  activeSidebarSection: SidebarSection;

  // Loading states.
  globalLoading: boolean;

  // Search state.
  searchQuery: string;
  searchOpen: boolean;

  // Actions for modals.
  openModal: (type: ModalType, data?: ModalData) => void;
  closeModal: () => void;

  // Actions for toasts.
  addToast: (toast: Omit<Toast, 'id'>) => void;
  removeToast: (id: string) => void;
  clearToasts: () => void;

  // Actions for sidebar.
  toggleSidebar: () => void;
  setSidebarSection: (section: SidebarSection) => void;

  // Actions for loading.
  setGlobalLoading: (loading: boolean) => void;

  // Actions for search.
  setSearchQuery: (query: string) => void;
  toggleSearch: () => void;
  closeSearch: () => void;
}

// Generate unique ID for toasts.
function generateToastId(): string {
  return `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export const useUIStore = create<UIState>()(
  devtools(
    (set) => ({
      // Initial state.
      activeModal: null,
      modalData: null,
      toasts: [],
      sidebarCollapsed: false,
      activeSidebarSection: 'inbox',
      globalLoading: false,
      searchQuery: '',
      searchOpen: false,

      // Modal actions.
      openModal: (type, data) =>
        set(
          { activeModal: type, modalData: data ?? null },
          undefined,
          'openModal',
        ),

      closeModal: () =>
        set({ activeModal: null, modalData: null }, undefined, 'closeModal'),

      // Toast actions.
      addToast: (toast) =>
        set(
          (state) => ({
            toasts: [
              ...state.toasts,
              { id: generateToastId(), duration: 5000, ...toast },
            ],
          }),
          undefined,
          'addToast',
        ),

      removeToast: (id) =>
        set(
          (state) => ({
            toasts: state.toasts.filter((t) => t.id !== id),
          }),
          undefined,
          'removeToast',
        ),

      clearToasts: () => set({ toasts: [] }, undefined, 'clearToasts'),

      // Sidebar actions.
      toggleSidebar: () =>
        set(
          (state) => ({ sidebarCollapsed: !state.sidebarCollapsed }),
          undefined,
          'toggleSidebar',
        ),

      setSidebarSection: (section) =>
        set({ activeSidebarSection: section }, undefined, 'setSidebarSection'),

      // Loading actions.
      setGlobalLoading: (loading) =>
        set({ globalLoading: loading }, undefined, 'setGlobalLoading'),

      // Search actions.
      setSearchQuery: (query) =>
        set({ searchQuery: query }, undefined, 'setSearchQuery'),

      toggleSearch: () =>
        set(
          (state) => ({ searchOpen: !state.searchOpen }),
          undefined,
          'toggleSearch',
        ),

      closeSearch: () =>
        set({ searchOpen: false, searchQuery: '' }, undefined, 'closeSearch'),
    }),
    { name: 'ui-store' },
  ),
);
