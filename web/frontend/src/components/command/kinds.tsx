// Visual configuration for command canvas event kinds. The palette is
// ink-on-paper with quiet tinted chips; classification drives weight,
// not decoration.
//
// Named palette:
//   paper    #F7F6F3  canvas surface
//   card     #FFFFFF  card surface
//   ink      #22262A  primary text
//   graphite #6B7280  secondary text
//   hairline #E6E4DD  borders
//   signal   #178A5B  live / approved
//   hold     #92610E  waiting / questions
//   flare    #B3372B  urgent
//   plan     #5B5BD6  plans
//   tide     #0E7490  diffs

import type { ReactNode } from 'react';
import type { AttentionKind, CommandEventKind } from '@/api/command.js';

// Per-kind visual treatment.
export interface KindConfig {
  label: string;
  // Accent text color for icons.
  text: string;
  // Left edge marker on expanded/actionable rows.
  stripe: string;
  // Tinted chip for the kind label.
  chip: string;
  icon: ReactNode;
}

// Small inline icons (14px), 1.5px strokes.
function PlanIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
    </svg>
  );
}

function DiffIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8 7h12M8 12h12M8 17h12M4 7h.01M4 12h.01M4 17h.01" />
    </svg>
  );
}

function QuestionIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  );
}

function StatusIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  );
}

function ReviewIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
    </svg>
  );
}

function MessageIcon() {
  return (
    <svg className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24"
      stroke="currentColor" strokeWidth={1.8}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}

// Kind configs for card event rows.
export const eventKinds: Record<CommandEventKind, KindConfig> = {
  plan: {
    label: 'Plan',
    text: 'text-[#5B5BD6]',
    stripe: 'border-l-[#5B5BD6]',
    chip: 'bg-[#5B5BD6]/10 text-[#4a4ab8]',
    icon: <PlanIcon />,
  },
  diff: {
    label: 'Diff',
    text: 'text-[#0E7490]',
    stripe: 'border-l-[#0E7490]',
    chip: 'bg-[#0E7490]/10 text-[#0E7490]',
    icon: <DiffIcon />,
  },
  question: {
    label: 'Needs input',
    text: 'text-[#92610E]',
    stripe: 'border-l-[#C98A1B]',
    chip: 'bg-[#C98A1B]/12 text-[#92610E]',
    icon: <QuestionIcon />,
  },
  status: {
    label: 'Status',
    text: 'text-[#6B7280]',
    stripe: 'border-l-[#D8D6CE]',
    chip: 'bg-[#6B7280]/8 text-[#6B7280]',
    icon: <StatusIcon />,
  },
  review: {
    label: 'Review',
    text: 'text-[#A13D63]',
    stripe: 'border-l-[#A13D63]',
    chip: 'bg-[#A13D63]/10 text-[#A13D63]',
    icon: <ReviewIcon />,
  },
  message: {
    label: 'Message',
    text: 'text-[#33608D]',
    stripe: 'border-l-[#33608D]',
    chip: 'bg-[#33608D]/10 text-[#33608D]',
    icon: <MessageIcon />,
  },
};

// Kind configs for the attention tray.
export const attentionKinds: Record<AttentionKind, KindConfig> = {
  plan: eventKinds.plan,
  question: eventKinds.question,
  urgent: {
    label: 'Urgent',
    text: 'text-[#B3372B]',
    stripe: 'border-l-[#B3372B]',
    chip: 'bg-[#B3372B]/10 text-[#B3372B]',
    icon: <StatusIcon />,
  },
  blocked: {
    label: 'Blocked',
    text: 'text-[#92610E]',
    stripe: 'border-l-[#C98A1B]',
    chip: 'bg-[#C98A1B]/12 text-[#92610E]',
    icon: <QuestionIcon />,
  },
};

// Relative time formatting shared by command components.
export function timeAgo(iso: string): string {
  if (!iso) {
    return '';
  }

  const seconds = Math.max(
    0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000),
  );
  if (seconds < 60) {
    return 'now';
  }
  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)}m`;
  }
  if (seconds < 86400) {
    return `${Math.floor(seconds / 3600)}h`;
  }
  return `${Math.floor(seconds / 86400)}d`;
}
