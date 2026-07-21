// Sidebar component - main navigation sidebar with nav links and actions.

import { type ReactNode, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { useUIStore, type SidebarSection } from '@/stores/ui.js';
import { useCanvasStore } from '@/stores/canvas.js';
import { useAgentsStatus } from '@/hooks/useAgents.js';
import { useMessages } from '@/hooks/useMessages.js';
import { routes } from '@/lib/routes.js';
import { HeartbeatTrace } from '@/components/command/HeartbeatTrace.js';

// Combine clsx and tailwind-merge for class name handling.
function cn(...inputs: (string | undefined | null | false)[]) {
  return twMerge(clsx(inputs));
}

// Navigation items for sidebar.
interface NavItem {
  id: SidebarSection;
  label: string;
  path: string;
  icon: ReactNode;
  badge?: number;
}

// Icon components for navigation.
function CommandIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 3H5a2 2 0 00-2 2v4m6-6h10a2 2 0 012 2v4M9 3v18m0 0h10a2 2 0 002-2V9M9 21H5a2 2 0 01-2-2V9m0 0h18"
      />
    </svg>
  );
}

function InboxIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
      />
    </svg>
  );
}

function UsersIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
      />
    </svg>
  );
}

function CodeReviewIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
      />
    </svg>
  );
}

function TasksIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
      />
    </svg>
  );
}

function PlansIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
      />
    </svg>
  );
}

function SettingsIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
      />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
    </svg>
  );
}

function ChevronDownIcon({ className }: { className?: string }) {
  return (
    <svg className={cn('h-4 w-4', className)} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  );
}

// CollapseIcon points into the sidebar edge: « to shrink, » to grow.
function CollapseIcon({ expand = false }: { expand?: boolean }) {
  return (
    <svg
      className={cn('h-4 w-4', expand && 'rotate-180')}
      fill="none" viewBox="0 0 24 24" stroke="currentColor"
    >
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
        d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
    </svg>
  );
}

function SmallPlusIcon() {
  return (
    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v12m6-6H6" />
    </svg>
  );
}

// Default navigation items. Sent lives inside Signals as a folder
// now that timelines are two-way everywhere.
const navItems: NavItem[] = [
  { id: 'command', label: 'Canvas', path: routes.command, icon: <CommandIcon /> },
  { id: 'inbox', label: 'Signals', path: routes.inbox, icon: <InboxIcon /> },
  { id: 'agents', label: 'Fleet', path: routes.agents, icon: <UsersIcon /> },
  { id: 'reviews', label: 'Reviews', path: routes.reviews, icon: <CodeReviewIcon /> },
  { id: 'tasks', label: 'Tasks', path: routes.tasks, icon: <TasksIcon /> },
  { id: 'plans', label: 'Plans', path: routes.plans, icon: <PlansIcon /> },
];

// Logo component - hidden in sidebar since branding is in header now.
export interface LogoProps {
  collapsed?: boolean;
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function Logo(_props: LogoProps = {}) {
  // Logo is now shown in the header, so sidebar just has spacing.
  return <div className="h-2" />;
}

// Sidebar navigation link.
interface NavLinkProps {
  item: NavItem;
  isActive: boolean;
  collapsed?: boolean;
}

function NavLink({ item, isActive, collapsed = false }: NavLinkProps) {
  return (
    <Link
      to={item.path}
      className={cn(
        'flex items-center gap-2.5 rounded-lg px-2.5 py-[7px] text-[13px] font-medium transition-colors',
        '[&_svg]:h-[18px] [&_svg]:w-[18px]',
        isActive
          ? 'border border-[var(--c-hair)] bg-[var(--c-card)] font-semibold text-[var(--c-ink)] shadow-[0_1px_2px_rgba(28,32,36,0.06)]'
          : 'text-[var(--c-mut)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]',
        collapsed ? 'justify-center' : '',
      )}
      title={collapsed ? item.label : undefined}
    >
      <span className={cn(isActive ? 'text-[var(--c-ink)]' : 'text-[var(--c-dim)]')}>
        {item.icon}
      </span>
      {!collapsed ? (
        <>
          <span className="flex-1">{item.label}</span>
          {item.badge && item.badge > 0 ? (
            <span className="rounded-full bg-[#33608D]/10 px-1.5 py-px font-mono text-[10px] font-semibold text-[var(--c-steel)]">
              {item.badge}
            </span>
          ) : null}
        </>
      ) : null}
    </Link>
  );
}

// Collapsible section header.
interface SidebarSectionHeaderProps {
  label: string;
  icon: ReactNode;
  isExpanded: boolean;
  onToggle: () => void;
  count?: number;
  onAddClick?: () => void;
}

function SidebarSectionHeader({
  label,
  icon,
  isExpanded,
  onToggle,
  count,
  onAddClick,
}: SidebarSectionHeaderProps) {
  return (
    <div className="flex items-center justify-between px-2.5 py-1.5">
      <button
        type="button"
        onClick={onToggle}
        className="flex flex-1 items-center gap-1.5 font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--c-mut)] hover:text-[var(--c-ink)]"
      >
        <ChevronDownIcon
          className={cn(
            'h-3 w-3 text-[var(--c-dim)] transition-transform',
            !isExpanded && '-rotate-90',
          )}
        />
        {icon}
        <span>{label}</span>
        {count !== undefined && count > 0 ? (
          <span className="font-mono text-[10px] normal-case tracking-normal text-[var(--c-faint)]">
            {count}
          </span>
        ) : null}
      </button>
      {onAddClick ? (
        <button
          type="button"
          onClick={onAddClick}
          className="rounded p-1 text-[var(--c-dim)] hover:bg-[var(--c-fill)] hover:text-[var(--c-text2)]"
          title={`Add ${label.slice(0, -1)}`}
        >
          <SmallPlusIcon />
        </button>
      ) : null}
    </div>
  );
}

// Agent item in sidebar.
interface AgentItemProps {
  name: string;
  status: 'active' | 'busy' | 'idle' | 'offline';
  onClick?: () => void;
}

function AgentItem({ name, status, onClick }: AgentItemProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-center gap-2.5 rounded-lg px-3 py-1.5 text-sm text-[var(--c-mut)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
      title={status}
    >
      <HeartbeatTrace status={status} className="w-9 shrink-0" />
      <span className="flex-1 truncate text-left">{name}</span>
    </button>
  );
}

// Sidebar props.
export interface SidebarProps {
  /** Custom navigation items (overrides defaults). */
  navItems?: NavItem[];
  /** Whether to show the compose button. */
  showComposeButton?: boolean;
  /** Whether to show the settings link. */
  showSettings?: boolean;
  /** Additional class name. */
  className?: string;
  /** Custom footer content. */
  footer?: ReactNode;
}

// Determine active section from current path.
function useActiveSection(): SidebarSection {
  const location = useLocation();
  const path = location.pathname;

  if (path.startsWith('/command')) return 'command';
  if (path.startsWith('/agents')) return 'agents';
  if (path.startsWith('/reviews')) return 'reviews';
  if (path.startsWith('/tasks')) return 'tasks';
  if (path.startsWith('/plans')) return 'plans';
  if (path.startsWith('/sent')) return 'sent';
  return 'inbox';
}

// Main Sidebar component.
export function Sidebar({
  navItems: customNavItems,
  showComposeButton = true,
  showSettings = true,
  className,
  footer,
}: SidebarProps) {
  const openModal = useUIStore((state) => state.openModal);
  const sidebarCollapsed = useUIStore((state) => state.sidebarCollapsed);
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);
  const activeSection = useActiveSection();
  const navigate = useNavigate();
  const setFocusedCard = useCanvasStore((s) => s.setFocusedCard);

  // State for the collapsible live-agents section.
  const [agentsExpanded, setAgentsExpanded] = useState(true);

  const { data: agentsData } = useAgentsStatus();

  // Unread mail count badges the Signals row. Shares the header's
  // query, so no extra request.
  const { data: unreadData } = useMessages({ filter: 'unread' });
  const unreadCount = unreadData?.data?.length ?? 0;

  // liveAgents is the current working set: busy sessions first, then
  // recently active ones.
  const liveAgents = useMemo(() => {
    const live = (agentsData?.agents ?? []).filter(
      (a) => a.status === 'active' || a.status === 'busy',
    );
    return live.sort((a, b) => {
      if (a.status !== b.status) {
        return a.status === 'busy' ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });
  }, [agentsData?.agents]);

  // handleAgentClick jumps to the agent's dossier on the canvas.
  const handleAgentClick = (agentId: number) => {
    setFocusedCard(agentId);
    navigate(routes.command);
  };

  const items = (customNavItems ?? navItems).map((it) =>
    it.id === 'inbox' && unreadCount > 0
      ? { ...it, badge: unreadCount }
      : it,
  );

  // Collapsed: render the thin icon strip instead of disappearing.
  if (sidebarCollapsed) {
    return <CollapsedSidebar className={className} />;
  }

  return (
    <aside
      className={cn(
        'flex h-full w-64 flex-col border-r border-[var(--c-hair)] bg-[var(--c-paper)]',
        className,
      )}
    >
      <Logo />

      {showComposeButton ? (
        <div className="px-3 pb-2 pt-3">
          <button
            type="button"
            onClick={() => openModal('compose')}
            className={cn(
              'flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5',
              'bg-[var(--c-ink)] text-[13.5px] font-medium text-white',
              'hover:bg-[var(--c-inkhover)] transition-colors',
              'focus:outline-none focus:ring-2 focus:ring-[#22262A]/30 focus:ring-offset-2',
              '[&_svg]:h-4 [&_svg]:w-4',
            )}
          >
            <PlusIcon />
            <span>Compose</span>
          </button>
        </div>
      ) : null}

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-2">
        {items.map((item) => (
          <NavLink
            key={item.id}
            item={item}
            isActive={activeSection === item.id}
          />
        ))}

        {/* Live agents: the working set right now. Clicking one jumps
            to its card on the canvas in focus mode. */}
        <div className="mt-4 border-t border-[var(--c-hair2)] pt-2">
          <SidebarSectionHeader
            label="Live"
            icon={
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  liveAgents.length > 0
                    ? 'animate-pulse-dot bg-[var(--c-green)]'
                    : 'bg-[var(--c-ghost)]',
                )}
              />
            }
            isExpanded={agentsExpanded}
            onToggle={() => setAgentsExpanded(!agentsExpanded)}
            {...(liveAgents.length > 0 && { count: liveAgents.length })}
            onAddClick={() => openModal('newAgent')}
          />
          {agentsExpanded ? (
            <div className="ml-2 space-y-0.5">
              {liveAgents.length > 0 ? (
                liveAgents.slice(0, 8).map((agent) => (
                  <AgentItem
                    key={agent.id}
                    name={agent.name}
                    status={agent.status}
                    onClick={() => handleAgentClick(agent.id)}
                  />
                ))
              ) : (
                <p className="px-3 py-2 text-xs text-[var(--c-dim)]">
                  No live agents
                </p>
              )}
            </div>
          ) : null}
        </div>
      </nav>

      {footer}

      <div className="flex items-center gap-1 border-t border-[var(--c-hair)] p-3">
        {showSettings ? (
          <Link
            to={routes.settings}
            className="flex flex-1 items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-[var(--c-mut)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
          >
            <span className="text-[var(--c-dim)]">
              <SettingsIcon />
            </span>
            <span>Settings</span>
          </Link>
        ) : (
          <div className="flex-1" />
        )}
        <button
          type="button"
          onClick={toggleSidebar}
          title="Collapse sidebar"
          aria-label="Collapse sidebar"
          className="rounded-lg p-2 text-[var(--c-dim)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
        >
          <CollapseIcon />
        </button>
      </div>
    </aside>
  );
}

// CollapsedSidebar is the thin icon strip: compose, icon-only nav
// with tooltips, then settings and the expand control at the foot.
export function CollapsedSidebar({
  className,
}: {
  className?: string | undefined;
}) {
  const activeSection = useActiveSection();
  const openModal = useUIStore((state) => state.openModal);
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);

  return (
    <aside
      className={cn(
        'flex h-full w-14 flex-col items-center border-r border-[var(--c-hair)] bg-[var(--c-paper)]',
        className,
      )}
    >
      <div className="py-3">
        <button
          type="button"
          onClick={() => openModal('compose')}
          className={cn(
            'flex h-9 w-9 items-center justify-center rounded-xl',
            'bg-[var(--c-ink)] text-white hover:bg-[var(--c-inkhover)]',
            'focus:outline-none focus:ring-2 focus:ring-[#22262A]/30',
          )}
          aria-label="Compose"
          title="Compose"
        >
          <PlusIcon />
        </button>
      </div>

      <nav className="flex flex-1 flex-col items-center gap-1 py-1">
        {navItems.map((item) => (
          <Link
            key={item.id}
            to={item.path}
            title={item.label}
            aria-label={item.label}
            className={cn(
              'flex h-9 w-9 items-center justify-center rounded-lg transition-colors',
              activeSection === item.id
                ? 'border border-[var(--c-hair)] bg-[var(--c-card)] text-[var(--c-ink)] shadow-[0_1px_2px_rgba(28,32,36,0.06)]'
                : 'text-[var(--c-dim)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]',
            )}
          >
            {item.icon}
          </Link>
        ))}
      </nav>

      <div className="flex flex-col items-center gap-1 border-t border-[var(--c-hair)] py-2">
        <Link
          to={routes.settings}
          className="flex h-9 w-9 items-center justify-center rounded-lg text-[var(--c-dim)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
          aria-label="Settings"
          title="Settings"
        >
          <SettingsIcon />
        </Link>
        <button
          type="button"
          onClick={toggleSidebar}
          title="Expand sidebar"
          aria-label="Expand sidebar"
          className="flex h-9 w-9 items-center justify-center rounded-lg text-[var(--c-dim)] hover:bg-[var(--c-fill)] hover:text-[var(--c-ink)]"
        >
          <CollapseIcon expand />
        </button>
      </div>
    </aside>
  );
}
