// Visual configuration for command feed event kinds: accent colors,
// labels, and icons shared by lanes and the attention rail.

import type { ReactNode } from 'react';
import type { AttentionKind, CommandEventKind } from '@/api/command.js';

// Per-kind visual treatment. Colors are Tailwind classes tuned for the
// dark command center surface.
export interface KindConfig {
  label: string;
  // Text accent for icons / labels.
  text: string;
  // Left border stripe for event cards.
  stripe: string;
  // Chip background for badges.
  chip: string;
  icon: ReactNode;
}

// Small inline icons (14px) used in event rows and attention items.
function PlanIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
    </svg>
  );
}

function DiffIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8 7h12M8 12h12M8 17h12M4 7h.01M4 12h.01M4 17h.01" />
    </svg>
  );
}

function QuestionIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function StatusIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  );
}

function ReviewIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
    </svg>
  );
}

function MessageIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}

// Kind configs for lane event cards.
export const eventKinds: Record<CommandEventKind, KindConfig> = {
  plan: {
    label: 'Plan',
    text: 'text-violet-300',
    stripe: 'border-l-violet-400',
    chip: 'bg-violet-500/15 text-violet-300',
    icon: <PlanIcon />,
  },
  diff: {
    label: 'Diff',
    text: 'text-cyan-300',
    stripe: 'border-l-cyan-400',
    chip: 'bg-cyan-500/15 text-cyan-300',
    icon: <DiffIcon />,
  },
  question: {
    label: 'Needs input',
    text: 'text-amber-300',
    stripe: 'border-l-amber-400',
    chip: 'bg-amber-500/15 text-amber-300',
    icon: <QuestionIcon />,
  },
  status: {
    label: 'Status',
    text: 'text-slate-400',
    stripe: 'border-l-slate-600',
    chip: 'bg-slate-500/15 text-slate-400',
    icon: <StatusIcon />,
  },
  review: {
    label: 'Review',
    text: 'text-rose-300',
    stripe: 'border-l-rose-400',
    chip: 'bg-rose-500/15 text-rose-300',
    icon: <ReviewIcon />,
  },
  message: {
    label: 'Message',
    text: 'text-sky-300',
    stripe: 'border-l-sky-500',
    chip: 'bg-sky-500/15 text-sky-300',
    icon: <MessageIcon />,
  },
};

// Kind configs for the attention rail (subset with blocked marker).
export const attentionKinds: Record<AttentionKind, KindConfig> = {
  plan: eventKinds.plan,
  question: eventKinds.question,
  urgent: {
    label: 'Urgent',
    text: 'text-red-300',
    stripe: 'border-l-red-400',
    chip: 'bg-red-500/15 text-red-300',
    icon: <StatusIcon />,
  },
  blocked: {
    label: 'Blocked',
    text: 'text-orange-300',
    stripe: 'border-l-orange-400',
    chip: 'bg-orange-500/15 text-orange-300',
    icon: <QuestionIcon />,
  },
};

// Status dot styling per agent liveness status.
export function statusDotClass(status: string): string {
  switch (status) {
    case 'busy':
      return 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)]';
    case 'active':
      return 'bg-emerald-400';
    case 'idle':
      return 'bg-amber-400';
    default:
      return 'bg-slate-600';
  }
}

// Relative time formatting shared by command components.
export function timeAgo(iso: string): string {
  if (!iso) {
    return '';
  }

  const seconds = Math.max(
    0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000),
  );
  if (seconds < 60) {
    return 'just now';
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m ago`;
  }
  if (seconds < 86400) {
    return `${Math.floor(seconds / 3600)}h ago`;
  }
  return `${Math.floor(seconds / 86400)}d ago`;
}
