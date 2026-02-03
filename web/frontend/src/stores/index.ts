// Re-export all stores for convenient imports.

export { useUIStore } from './ui.js';
export type {
  Toast,
  ToastVariant,
  ToastAction,
  ModalType,
  ModalData,
  SidebarSection,
} from './ui.js';

export { useAuthStore } from './auth.js';
export type { Agent, AgentStatus, AgentStatusType } from './auth.js';
