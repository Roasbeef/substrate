// Route path constants for navigation.
// Separated from router.tsx to avoid circular dependencies.

export const routes = {
  inbox: '/inbox',
  inboxCategory: (category: string) => `/inbox/${category}`,
  starred: '/starred',
  snoozed: '/snoozed',
  sent: '/sent',
  archive: '/archive',
  thread: (threadId: number | string) => `/thread/${threadId}`,
  agents: '/agents',
  agent: (agentId: number | string) => `/agents/${agentId}`,
  sessions: '/sessions',
  session: (sessionId: number | string) => `/sessions/${sessionId}`,
  reviews: '/reviews',
  review: (reviewId: string) => `/reviews/${reviewId}`,
  settings: '/settings',
  search: '/search',
} as const;
