// HeartbeatTrace is the canvas's signature element: a small EKG-style
// trace beside each agent's name. Active agents beat, idle agents show
// a slow shallow pulse, offline agents flatline as a dotted rule.

import { clsx } from 'clsx';

export interface HeartbeatTraceProps {
  status: string;
  className?: string;
}

// One beat of an EKG waveform sized to a 64x16 viewBox.
const BEAT_PATH =
  'M0 8 H14 L18 8 L21 3 L25 13 L28 8 H40 L44 8 L47 5 L50 11 L52 8 H64';

export function HeartbeatTrace({
  status,
  className,
}: HeartbeatTraceProps) {
  const live = status === 'active' || status === 'busy';
  const idle = status === 'idle';

  if (!live && !idle) {
    // Offline: a quiet dotted flatline.
    return (
      <svg
        className={clsx('h-4 w-16', className)}
        viewBox="0 0 64 16"
        aria-label="offline"
      >
        <path
          d="M0 8 H64"
          fill="none"
          stroke="#C9C7BF"
          strokeWidth="1.5"
          strokeDasharray="2 4"
          strokeLinecap="round"
        />
      </svg>
    );
  }

  return (
    <svg
      className={clsx('h-4 w-16', className)}
      viewBox="0 0 64 16"
      aria-label={live ? 'active' : 'idle'}
    >
      <path
        className={live ? 'trace-beat' : 'trace-idle'}
        d={live ? BEAT_PATH : 'M0 8 H24 L28 6 L32 10 L36 8 H64'}
        fill="none"
        stroke={live ? '#178A5B' : '#B9A15C'}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
