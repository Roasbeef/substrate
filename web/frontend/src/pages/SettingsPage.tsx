// Settings page with notification preferences and agent configuration.

import { useState } from 'react';
import { NotificationSettings } from '@/components/layout/NotificationSettings';
import { useAuthStore } from '@/stores/auth';
import { Button } from '@/components/ui/Button';
import { Input, Textarea } from '@/components/ui/Input';
import { Badge } from '@/components/ui/Badge';

// Inline SVG icon components.
function BellIcon({ className = '' }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
      />
    </svg>
  );
}

function UserIcon({ className = '' }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  );
}

function CogIcon({ className = '' }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
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

function KeyIcon({ className = '' }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
      />
    </svg>
  );
}

type SettingsTab = 'notifications' | 'agent' | 'appearance' | 'api';

interface TabConfig {
  id: SettingsTab;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}

// SettingsPage provides a comprehensive settings interface.
export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('notifications');

  const tabs: TabConfig[] = [
    { id: 'notifications', label: 'Notifications', icon: BellIcon },
    { id: 'agent', label: 'Agent Profile', icon: UserIcon },
    { id: 'appearance', label: 'Appearance', icon: CogIcon },
    { id: 'api', label: 'API & Tokens', icon: KeyIcon },
  ];

  return (
    <div className="mx-auto max-w-4xl p-6">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="mt-1 text-sm text-gray-600">
          Manage your preferences and agent configuration.
        </p>
      </div>

      <div className="flex gap-8">
        {/* Sidebar navigation. */}
        <nav className="w-48 flex-shrink-0">
          <ul className="space-y-1">
            {tabs.map((tab) => {
              const Icon = tab.icon;
              const isActive = activeTab === tab.id;
              return (
                <li key={tab.id}>
                  <button
                    onClick={() => setActiveTab(tab.id)}
                    className={`
                      flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors
                      ${isActive
                        ? 'bg-blue-50 text-blue-700'
                        : 'text-gray-700 hover:bg-gray-100'
                      }
                    `}
                  >
                    <Icon className="h-5 w-5" />
                    {tab.label}
                  </button>
                </li>
              );
            })}
          </ul>
        </nav>

        {/* Settings content. */}
        <div className="flex-1">
          <div className="rounded-lg border border-gray-200 bg-white p-6">
            {activeTab === 'notifications' && <NotificationsSection />}
            {activeTab === 'agent' && <AgentProfileSection />}
            {activeTab === 'appearance' && <AppearanceSection />}
            {activeTab === 'api' && <ApiTokensSection />}
          </div>
        </div>
      </div>
    </div>
  );
}

// Notifications settings section.
function NotificationsSection() {
  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">Notifications</h2>
      <p className="mb-6 text-sm text-gray-600">
        Configure how you receive notifications about new messages and agent activity.
      </p>
      <NotificationSettings />
    </div>
  );
}

// Agent profile settings section.
function AgentProfileSection() {
  const { currentAgent } = useAuthStore();
  const [displayName, setDisplayName] = useState(currentAgent?.name || '');
  const [bio, setBio] = useState('');
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = async () => {
    setIsSaving(true);
    // Simulate save.
    await new Promise((resolve) => setTimeout(resolve, 1000));
    setIsSaving(false);
  };

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">Agent Profile</h2>
      <p className="mb-6 text-sm text-gray-600">
        Customize how your agent appears to others in the system.
      </p>

      <div className="space-y-6">
        {/* Current agent info. */}
        {currentAgent && (
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex items-center gap-3">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-blue-100 text-blue-600">
                <UserIcon className="h-6 w-6" />
              </div>
              <div>
                <p className="font-medium text-gray-900">{currentAgent.name}</p>
                <p className="text-sm text-gray-500">Agent ID: {currentAgent.id}</p>
              </div>
              <Badge variant="success" className="ml-auto">
                Active
              </Badge>
            </div>
          </div>
        )}

        {/* Display name. */}
        <div>
          <Input
            label="Display Name"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Enter agent display name"
          />
          <p className="mt-1 text-xs text-gray-500">
            This name will be shown to other agents in messages.
          </p>
        </div>

        {/* Bio/description. */}
        <div>
          <Textarea
            label="Description"
            value={bio}
            onChange={(e) => setBio(e.target.value)}
            placeholder="Brief description of what this agent does..."
            rows={3}
          />
        </div>

        {/* Save button. */}
        <div className="flex justify-end">
          <Button variant="primary" onClick={handleSave} isLoading={isSaving}>
            Save Changes
          </Button>
        </div>
      </div>
    </div>
  );
}

// Appearance settings section.
function AppearanceSection() {
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>('light');
  const [density, setDensity] = useState<'comfortable' | 'compact'>('comfortable');

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">Appearance</h2>
      <p className="mb-6 text-sm text-gray-600">
        Customize the look and feel of the interface.
      </p>

      <div className="space-y-6">
        {/* Theme selection. */}
        <div>
          <label className="block text-sm font-medium text-gray-700">Theme</label>
          <div className="mt-2 flex gap-3">
            {(['light', 'dark', 'system'] as const).map((t) => (
              <button
                key={t}
                onClick={() => setTheme(t)}
                className={`
                  rounded-lg border px-4 py-2 text-sm font-medium transition-colors
                  ${theme === t
                    ? 'border-blue-600 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-700 hover:bg-gray-50'
                  }
                `}
              >
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </button>
            ))}
          </div>
          <p className="mt-1 text-xs text-gray-500">
            System theme will follow your OS preferences.
          </p>
        </div>

        {/* Density selection. */}
        <div>
          <label className="block text-sm font-medium text-gray-700">Display Density</label>
          <div className="mt-2 flex gap-3">
            {(['comfortable', 'compact'] as const).map((d) => (
              <button
                key={d}
                onClick={() => setDensity(d)}
                className={`
                  rounded-lg border px-4 py-2 text-sm font-medium transition-colors
                  ${density === d
                    ? 'border-blue-600 bg-blue-50 text-blue-700'
                    : 'border-gray-300 text-gray-700 hover:bg-gray-50'
                  }
                `}
              >
                {d.charAt(0).toUpperCase() + d.slice(1)}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// API tokens settings section.
function ApiTokensSection() {
  const [showToken, setShowToken] = useState(false);
  const exampleToken = 'sk_live_xxxxxxxxxxxxxxxxxxxxxxxxxx';

  return (
    <div>
      <h2 className="mb-4 text-lg font-semibold text-gray-900">API & Tokens</h2>
      <p className="mb-6 text-sm text-gray-600">
        Manage API access tokens for external integrations.
      </p>

      <div className="space-y-6">
        {/* API endpoint info. */}
        <div className="rounded-lg bg-gray-50 p-4">
          <h3 className="text-sm font-medium text-gray-900">API Endpoint</h3>
          <code className="mt-1 block text-sm text-gray-600">
            {window.location.origin}/api/v1
          </code>
        </div>

        {/* API token. */}
        <div>
          <label className="block text-sm font-medium text-gray-700">API Token</label>
          <div className="mt-2 flex gap-2">
            <div className="relative flex-1">
              <input
                type={showToken ? 'text' : 'password'}
                value={exampleToken}
                readOnly
                className="block w-full rounded-lg border border-gray-300 bg-gray-50 px-3 py-2 text-sm font-mono text-gray-900"
              />
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowToken(!showToken)}
            >
              {showToken ? 'Hide' : 'Show'}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => navigator.clipboard.writeText(exampleToken)}
            >
              Copy
            </Button>
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Keep this token secret. Never share it publicly.
          </p>
        </div>

        {/* Regenerate token. */}
        <div className="border-t border-gray-200 pt-4">
          <h3 className="text-sm font-medium text-gray-900">Regenerate Token</h3>
          <p className="mt-1 text-sm text-gray-600">
            Generate a new API token. This will invalidate your current token.
          </p>
          <Button variant="danger" size="sm" className="mt-3">
            Regenerate Token
          </Button>
        </div>
      </div>
    </div>
  );
}
