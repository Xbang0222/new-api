/*
Copyright (C) 2025 QuantumNous

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

import { useMemo } from 'react';

const getDefaultModules = () => ({
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  docs: true,
});

const normalizeHeaderNavModules = (headerNavModules) => {
  const defaults = getDefaultModules();
  if (!headerNavModules || typeof headerNavModules !== 'object') {
    return defaults;
  }

  const normalizedModules = {
    ...defaults,
    console:
      typeof headerNavModules.console === 'boolean'
        ? headerNavModules.console
        : defaults.console,
    docs:
      typeof headerNavModules.docs === 'boolean'
        ? headerNavModules.docs
        : defaults.docs,
  };

  if (typeof headerNavModules.pricing === 'boolean') {
    normalizedModules.pricing = {
      enabled: headerNavModules.pricing,
      requireAuth: false,
    };
  } else if (
    headerNavModules.pricing &&
    typeof headerNavModules.pricing === 'object'
  ) {
    normalizedModules.pricing = {
      enabled:
        typeof headerNavModules.pricing.enabled === 'boolean'
          ? headerNavModules.pricing.enabled
          : defaults.pricing.enabled,
      requireAuth:
        typeof headerNavModules.pricing.requireAuth === 'boolean'
          ? headerNavModules.pricing.requireAuth
          : defaults.pricing.requireAuth,
    };
  }

  return normalizedModules;
};

export const useNavigation = (t, docsLink, headerNavModules, pathname = '/') => {
  const mainNavLinks = useMemo(() => {
    const modules = normalizeHeaderNavModules(headerNavModules);
    const inConsoleArea =
      pathname.startsWith('/console') || pathname === '/pricing';

    const allLinks = [
      {
        text: t('控制台'),
        itemKey: 'console',
        to: '/console',
      },
      ...(docsLink
        ? [
            {
              text: t('文档'),
              itemKey: 'docs',
              isExternal: true,
              externalLink: docsLink,
            },
          ]
        : []),
      {
        text: t('模型广场'),
        itemKey: 'pricing',
        to: '/pricing',
      },
    ];

    const moduleFilteredLinks = allLinks.filter((link) => {
      if (link.itemKey === 'docs') {
        return docsLink && modules.docs;
      }
      if (link.itemKey === 'pricing') {
        return modules.pricing.enabled;
      }
      return modules[link.itemKey] === true;
    });

    return moduleFilteredLinks.map((link) => {
      const showOnDesktop = inConsoleArea
        ? link.itemKey !== 'console'
        : link.itemKey === 'console';
      return {
        ...link,
        showOnDesktop,
      };
    });
  }, [t, docsLink, headerNavModules, pathname]);

  return {
    mainNavLinks,
  };
};
