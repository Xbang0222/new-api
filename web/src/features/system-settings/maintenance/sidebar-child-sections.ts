/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type SidebarChildModule = {
  key: string
  titleKey: string
  url?: string
  defaultEnabled?: boolean
}

export type SidebarChildSection = {
  parentSection: string
  parentModule: string
  modules: readonly SidebarChildModule[]
}

// Major pages and content areas only; individual fields and table actions stay together.
export const SIDEBAR_CHILD_SECTIONS: Record<string, SidebarChildSection> = {
  overview: {
    parentSection: 'console',
    parentModule: 'overview',
    modules: [
      { key: 'summary', titleKey: 'Usage at a glance' },
      { key: 'community', titleKey: 'User community' },
      {
        key: 'invitationRanking',
        titleKey: 'Invitation earnings leaderboard',
        defaultEnabled: false,
      },
      { key: 'performanceHealth', titleKey: 'Performance health' },
      { key: 'announcements', titleKey: 'Announcements' },
      { key: 'apiInfo', titleKey: 'API Addresses' },
      { key: 'faq', titleKey: 'FAQ' },
      { key: 'uptimeKuma', titleKey: 'Uptime Kuma' },
      { key: 'guide', titleKey: 'View guide' },
    ],
  },
  overviewSummary: {
    parentSection: 'overview',
    parentModule: 'summary',
    modules: [
      { key: 'balance', titleKey: 'Credit remaining' },
      { key: 'todayUsage', titleKey: 'Last 24h usage' },
      { key: 'usage', titleKey: 'Historical Usage' },
      { key: 'requests', titleKey: 'Request Count' },
    ],
  },
  dashboard: {
    parentSection: 'console',
    parentModule: 'dashboard',
    modules: [
      {
        key: 'models',
        titleKey: 'Model Call Analytics',
        url: '/dashboard/models',
      },
      { key: 'flow', titleKey: 'Flow', url: '/dashboard/flow' },
      { key: 'users', titleKey: 'User Analytics', url: '/dashboard/users' },
    ],
  },
  taskLogs: {
    parentSection: 'console',
    parentModule: 'task',
    modules: [
      { key: 'task', titleKey: 'Task Logs', url: '/usage-logs/task' },
      { key: 'drawing', titleKey: 'Drawing Logs', url: '/usage-logs/drawing' },
    ],
  },
  wallet: {
    parentSection: 'personal',
    parentModule: 'topup',
    modules: [
      { key: 'summary', titleKey: 'Balance' },
      { key: 'recharge', titleKey: 'Add Funds' },
      { key: 'subscriptions', titleKey: 'Subscription Plans' },
      { key: 'affiliate', titleKey: 'Referral Program' },
    ],
  },
  profile: {
    parentSection: 'personal',
    parentModule: 'personal',
    modules: [
      { key: 'details', titleKey: 'Personal Settings' },
      { key: 'preferences', titleKey: 'Language Preferences' },
      { key: 'checkin', titleKey: 'Daily Check-in' },
      { key: 'sidebar', titleKey: 'Sidebar modules' },
    ],
  },
  security: {
    parentSection: 'personal',
    parentModule: 'security',
    modules: [
      { key: 'authentication', titleKey: 'Login & Authentication' },
      { key: 'sessions', titleKey: 'Sessions & Access' },
      { key: 'verification', titleKey: 'Security verification' },
      { key: 'privacy', titleKey: 'Privacy' },
      { key: 'account', titleKey: 'Account Actions' },
    ],
  },
  scheduling: {
    parentSection: 'admin',
    parentModule: 'channelScheduling',
    modules: [
      { key: 'overview', titleKey: 'Real-time overview' },
      { key: 'pools', titleKey: 'Pool details' },
      { key: 'settings', titleKey: 'Strategy settings' },
    ],
  },
  models: {
    parentSection: 'admin',
    parentModule: 'models',
    modules: [
      { key: 'metadata', titleKey: 'Models', url: '/models/metadata' },
      { key: 'vendors', titleKey: 'Vendors', url: '/models/vendors' },
      {
        key: 'deployments',
        titleKey: 'Deployments',
        url: '/models/deployments',
      },
    ],
  },
  systemInfo: {
    parentSection: 'admin',
    parentModule: 'systemInfo',
    modules: [
      { key: 'instances', titleKey: 'Instances' },
      { key: 'tasks', titleKey: 'System Tasks' },
    ],
  },
  taskPlugins: {
    parentSection: 'admin',
    parentModule: 'taskPlugins',
    modules: [
      { key: 'installed', titleKey: 'Installed' },
      { key: 'marketplace', titleKey: 'Marketplace' },
    ],
  },
  systemSettings: {
    parentSection: 'admin',
    parentModule: 'setting',
    modules: [
      {
        key: 'site',
        titleKey: 'Site & Branding',
        url: '/system-settings/site',
      },
      { key: 'auth', titleKey: 'Authentication', url: '/system-settings/auth' },
      {
        key: 'billing',
        titleKey: 'Billing & Payment',
        url: '/system-settings/billing',
      },
      { key: 'models', titleKey: 'Models', url: '/system-settings/models' },
      {
        key: 'request-policies',
        titleKey: 'Request policies',
        url: '/system-settings/request-policies',
      },
      {
        key: 'security',
        titleKey: 'Security & Limits',
        url: '/system-settings/security',
      },
      {
        key: 'content',
        titleKey: 'Console Content',
        url: '/system-settings/content',
      },
      {
        key: 'operations',
        titleKey: 'Operations',
        url: '/system-settings/operations',
      },
    ],
  },
}

export function getSidebarChildSection(section: string, module: string) {
  return Object.entries(SIDEBAR_CHILD_SECTIONS).find(
    ([, child]) =>
      child.parentSection === section && child.parentModule === module
  )
}
