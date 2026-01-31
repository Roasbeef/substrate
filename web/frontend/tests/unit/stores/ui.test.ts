// Unit tests for the UI store.

import { describe, it, expect, beforeEach } from 'vitest';
import { useUIStore } from '@/stores/ui';

describe('useUIStore', () => {
  beforeEach(() => {
    // Reset the store to initial state before each test.
    useUIStore.setState({
      activeModal: null,
      modalData: null,
      toasts: [],
      sidebarCollapsed: false,
      activeSidebarSection: 'inbox',
      globalLoading: false,
      searchQuery: '',
      searchOpen: false,
    });
  });

  describe('modal actions', () => {
    it('should open a modal with no data', () => {
      const store = useUIStore.getState();
      store.openModal('compose');

      const state = useUIStore.getState();
      expect(state.activeModal).toBe('compose');
      expect(state.modalData).toBeNull();
    });

    it('should open a modal with data', () => {
      const store = useUIStore.getState();
      store.openModal('thread', { threadId: 123 });

      const state = useUIStore.getState();
      expect(state.activeModal).toBe('thread');
      expect(state.modalData).toEqual({ threadId: 123 });
    });

    it('should close the modal and clear data', () => {
      const store = useUIStore.getState();
      store.openModal('compose', { replyTo: 456 });
      store.closeModal();

      const state = useUIStore.getState();
      expect(state.activeModal).toBeNull();
      expect(state.modalData).toBeNull();
    });
  });

  describe('toast actions', () => {
    it('should add a toast notification', () => {
      const store = useUIStore.getState();
      store.addToast({ variant: 'success', message: 'Message sent!' });

      const state = useUIStore.getState();
      expect(state.toasts).toHaveLength(1);
      expect(state.toasts[0]).toMatchObject({
        variant: 'success',
        message: 'Message sent!',
        duration: 5000,
      });
      expect(state.toasts[0]?.id).toMatch(/^toast-/);
    });

    it('should add multiple toasts', () => {
      const store = useUIStore.getState();
      store.addToast({ variant: 'success', message: 'First' });
      store.addToast({ variant: 'error', message: 'Second' });

      const state = useUIStore.getState();
      expect(state.toasts).toHaveLength(2);
    });

    it('should remove a specific toast', () => {
      const store = useUIStore.getState();
      store.addToast({ variant: 'success', message: 'First' });
      store.addToast({ variant: 'error', message: 'Second' });

      const toasts = useUIStore.getState().toasts;
      const firstToastId = toasts[0]?.id;
      if (firstToastId !== undefined) {
        store.removeToast(firstToastId);
      }

      const state = useUIStore.getState();
      expect(state.toasts).toHaveLength(1);
      expect(state.toasts[0]?.message).toBe('Second');
    });

    it('should clear all toasts', () => {
      const store = useUIStore.getState();
      store.addToast({ variant: 'success', message: 'First' });
      store.addToast({ variant: 'error', message: 'Second' });
      store.clearToasts();

      const state = useUIStore.getState();
      expect(state.toasts).toHaveLength(0);
    });

    it('should accept custom duration', () => {
      const store = useUIStore.getState();
      store.addToast({ variant: 'warning', message: 'Quick!', duration: 2000 });

      const state = useUIStore.getState();
      expect(state.toasts[0]?.duration).toBe(2000);
    });

    it('should accept title and action', () => {
      const store = useUIStore.getState();
      const onClick = () => {};
      store.addToast({
        variant: 'info',
        message: 'New message',
        title: 'Notification',
        action: { label: 'View', onClick },
      });

      const state = useUIStore.getState();
      expect(state.toasts[0]?.title).toBe('Notification');
      expect(state.toasts[0]?.action?.label).toBe('View');
    });
  });

  describe('sidebar actions', () => {
    it('should toggle sidebar collapsed state', () => {
      const store = useUIStore.getState();
      expect(useUIStore.getState().sidebarCollapsed).toBe(false);

      store.toggleSidebar();
      expect(useUIStore.getState().sidebarCollapsed).toBe(true);

      store.toggleSidebar();
      expect(useUIStore.getState().sidebarCollapsed).toBe(false);
    });

    it('should set active sidebar section', () => {
      const store = useUIStore.getState();
      store.setSidebarSection('agents');

      expect(useUIStore.getState().activeSidebarSection).toBe('agents');
    });
  });

  describe('loading actions', () => {
    it('should set global loading state', () => {
      const store = useUIStore.getState();
      store.setGlobalLoading(true);
      expect(useUIStore.getState().globalLoading).toBe(true);

      store.setGlobalLoading(false);
      expect(useUIStore.getState().globalLoading).toBe(false);
    });
  });

  describe('search actions', () => {
    it('should set search query', () => {
      const store = useUIStore.getState();
      store.setSearchQuery('test query');

      expect(useUIStore.getState().searchQuery).toBe('test query');
    });

    it('should toggle search open state', () => {
      const store = useUIStore.getState();
      expect(useUIStore.getState().searchOpen).toBe(false);

      store.toggleSearch();
      expect(useUIStore.getState().searchOpen).toBe(true);
    });

    it('should close search and clear query', () => {
      const store = useUIStore.getState();
      store.setSearchQuery('test');
      store.toggleSearch();
      store.closeSearch();

      const state = useUIStore.getState();
      expect(state.searchOpen).toBe(false);
      expect(state.searchQuery).toBe('');
    });
  });
});
