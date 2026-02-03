// ModalContainer component - renders global modals based on UI store state.

import { useUIStore } from '@/stores/ui.js';
import { NewAgentModal } from '@/components/agents/NewAgentModal.js';
import { ComposeModal } from '@/components/inbox/ComposeModal.js';
import { SearchBar } from '@/components/layout/SearchBar.js';
import { useCreateAgent } from '@/hooks/useAgents.js';
import { useSendMessage } from '@/hooks/useMessages.js';
import { autocompleteRecipients } from '@/api/search.js';
import type { SendMessageRequest } from '@/types/api.js';

// Global modal container that renders the active modal.
export function ModalContainer() {
  const { activeModal, closeModal, addToast } = useUIStore();
  const createAgent = useCreateAgent();
  const sendMessage = useSendMessage();

  // Handler for creating a new agent.
  const handleCreateAgent = async (data: { name: string }) => {
    try {
      await createAgent.mutateAsync(data);
      addToast({
        variant: 'success',
        title: 'Agent Created',
        message: `Agent "${data.name}" was registered successfully.`,
      });
      closeModal();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to create agent';
      throw new Error(message);
    }
  };

  // Handler for sending a message.
  const handleSendMessage = async (data: SendMessageRequest) => {
    try {
      await sendMessage.mutateAsync(data);
      addToast({
        variant: 'success',
        title: 'Message Sent',
        message: 'Your message was sent successfully.',
      });
      closeModal();
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to send message';
      addToast({
        variant: 'error',
        title: 'Send Failed',
        message,
      });
      throw new Error(message);
    }
  };

  // Handler for searching recipients.
  const handleSearchRecipients = async (query: string) => {
    if (!query.trim()) {
      return [];
    }
    try {
      return await autocompleteRecipients(query);
    } catch {
      return [];
    }
  };

  // Render the active modal based on type.
  // Note: SearchBar is always rendered since it manages its own visibility via
  // the searchOpen state.
  return (
    <>
      {/* Global search modal - always rendered, visibility controlled by searchOpen state. */}
      <SearchBar />

      {/* Other modals based on activeModal state. */}
      {activeModal === 'compose' ? (
        <ComposeModal
          isOpen
          onClose={closeModal}
          onSend={handleSendMessage}
          onSearchRecipients={handleSearchRecipients}
          isSending={sendMessage.isPending}
        />
      ) : null}
      {activeModal === 'newAgent' ? (
        <NewAgentModal
          isOpen
          onClose={closeModal}
          onSubmit={handleCreateAgent}
          isSubmitting={createAgent.isPending}
          {...(createAgent.error?.message != null && { submitError: createAgent.error.message })}
        />
      ) : null}
    </>
  );
}
