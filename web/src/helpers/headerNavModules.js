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

const getDefaultPricingConfig = () => ({
  enabled: true,
  requireAuth: false,
});

export const getDefaultHeaderNavModules = () => ({
  console: true,
  pricing: getDefaultPricingConfig(),
  docs: true,
});

export const normalizeHeaderNavModules = (headerNavModules) => {
  const defaults = getDefaultHeaderNavModules();

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

export const parseHeaderNavModules = (headerNavModulesConfig) => {
  if (!headerNavModulesConfig) {
    return getDefaultHeaderNavModules();
  }

  if (typeof headerNavModulesConfig === 'string') {
    try {
      return normalizeHeaderNavModules(JSON.parse(headerNavModulesConfig));
    } catch (error) {
      console.error('解析顶栏模块配置失败:', error);
      return getDefaultHeaderNavModules();
    }
  }

  return normalizeHeaderNavModules(headerNavModulesConfig);
};

export const getPricingRequireAuth = (headerNavModules) => {
  const normalizedModules = normalizeHeaderNavModules(headerNavModules);
  return normalizedModules.pricing.requireAuth === true;
};
