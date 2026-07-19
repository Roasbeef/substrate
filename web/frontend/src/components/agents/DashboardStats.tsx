// DashboardStats component - displays agent status statistics.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { AgentStatusCounts } from '@/types/api.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Icon components.
function UsersIcon({ className }: { className?: string }) {
  return (
    <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z"
      />
    </svg>
  );
}

function BoltIcon({ className }: { className?: string }) {
  return (
    <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M13 10V3L4 14h7v7l9-11h-7z"
      />
    </svg>
  );
}

function ClockIcon({ className }: { className?: string }) {
  return (
    <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

function ServerIcon({ className }: { className?: string }) {
  return (
    <svg className={cn('h-5 w-5', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2"
      />
    </svg>
  );
}

// Single stat card.
interface StatCardProps {
  label: string;
  value: number;
  icon: React.ReactNode;
  iconBg: string;
  iconColor: string;
  onClick?: () => void;
}

function StatCard({ label, value, icon, iconBg, iconColor, onClick }: StatCardProps) {
  const Wrapper = onClick ? 'button' : 'div';

  return (
    <Wrapper
      onClick={onClick}
      className={cn(
        'flex items-center gap-4 rounded-lg border border-[#E6E4DD] bg-white p-4',
        onClick ? 'cursor-pointer text-left hover:border-[#D8D6CE] hover:shadow-sm transition-all' : '',
        onClick ? 'focus:outline-none focus:ring-2 focus:ring-[#22262A]/30 focus:ring-offset-2' : '',
      )}
    >
      <div
        className={cn(
          'flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg',
          iconBg,
          iconColor,
        )}
      >
        {icon}
      </div>
      <div>
        <p className="text-sm font-medium text-[#6B7280]">{label}</p>
        <p className="text-2xl font-semibold text-[#22262A]">{value}</p>
      </div>
    </Wrapper>
  );
}

// Props for DashboardStats component.
export interface DashboardStatsProps {
  /** Agent status counts. */
  counts?: AgentStatusCounts;
  /** Whether data is loading. */
  isLoading?: boolean;
  /** Handler for clicking a stat. */
  onStatClick?: (status: keyof AgentStatusCounts) => void;
  /** Additional class name. */
  className?: string;
}

export function DashboardStats({
  counts,
  isLoading = false,
  onStatClick,
  className,
}: DashboardStatsProps) {
  if (isLoading) {
    return (
      <div className={cn('grid grid-cols-2 gap-4 md:grid-cols-4', className)}>
        {[1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className="h-20 animate-pulse rounded-lg border border-[#E6E4DD] bg-[#F1EFE9]"
          />
        ))}
      </div>
    );
  }

  const total =
    (counts?.active ?? 0) +
    (counts?.busy ?? 0) +
    (counts?.idle ?? 0) +
    (counts?.offline ?? 0);

  return (
    <div className={cn('grid grid-cols-2 gap-4 md:grid-cols-4', className)}>
      <StatCard
        label="Total Agents"
        value={total}
        icon={<UsersIcon />}
        iconBg="bg-[#F1EFE9]"
        iconColor="text-[#6B7280]"
      />
      <StatCard
        label="Active"
        value={counts?.active ?? 0}
        icon={<BoltIcon />}
        iconBg="bg-[#F1EFE9]"
        iconColor="text-[#178A5B]"
        {...(onStatClick && { onClick: () => onStatClick('active') })}
      />
      <StatCard
        label="Busy"
        value={counts?.busy ?? 0}
        icon={<ServerIcon />}
        iconBg="bg-[#F1EFE9]"
        iconColor="text-[#92610E]"
        {...(onStatClick && { onClick: () => onStatClick('busy') })}
      />
      <StatCard
        label="Idle"
        value={counts?.idle ?? 0}
        icon={<ClockIcon />}
        iconBg="bg-[#F1EFE9]"
        iconColor="text-[#6B7280]"
        {...(onStatClick && { onClick: () => onStatClick('idle') })}
      />
    </div>
  );
}
