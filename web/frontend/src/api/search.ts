// API functions for search and autocomplete.

import { get } from './client.js';
import type {
  APIResponse,
  SearchResult,
  AutocompleteRecipient,
} from '@/types/api.js';

// Search across messages, threads, agents, and topics.
export function search(
  query: string,
  signal?: AbortSignal,
): Promise<APIResponse<SearchResult[]>> {
  const encoded = encodeURIComponent(query);
  return get<APIResponse<SearchResult[]>>(`/search?q=${encoded}`, signal);
}

// Autocomplete recipients (agents).
export function autocompleteRecipients(
  query: string,
  signal?: AbortSignal,
): Promise<AutocompleteRecipient[]> {
  const encoded = encodeURIComponent(query);
  return get<AutocompleteRecipient[]>(
    `/autocomplete/recipients?q=${encoded}`,
    signal,
  );
}
