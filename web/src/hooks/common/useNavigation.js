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
import { normalizeHeaderNavModules } from '../../helpers/headerNavModules';

// custom: shop link — defense in depth：拒绝 javascript:/data:/vbscript: 等
// 危险 scheme 的外链（防止管理员误填或被注入恶意 URL 后影响普通用户）。
// 同时覆盖 docsLink 和 shopLink 的入口校验。
const isSafeExternalUrl = (url) =>
  typeof url === 'string' && /^https?:\/\//i.test(url.trim());

// custom: shop link — 桌面端 nav link 排序表
// Original (upstream): 仅 console / docs / pricing 三档，按当前页面动态决定 0/1/2 优先级。
// Changed:  插入 'shop' 在 console 之后、docs 之前，所有分支都补成 4 档；查表替换嵌套 ternary。
// Revert:   删除 ORDER_BY_PAGE.shop 字段及表里的 shop 槽位，把 console/pricing 分支恢复成 3 档；
//           同时移除 allLinks 中的 shop spread、filter 中的 'shop' 分支、依赖数组里的 shopLink。
const ORDER_BY_PAGE = {
  // 控制台路径下：console 自身被 showOnDesktop=false 隐藏，剩余三个按此顺序展示
  console: { shop: 0, docs: 1, pricing: 2 },
  // 模型广场路径下：pricing 隐藏，剩余三个按此顺序展示
  pricing: { shop: 0, docs: 1, console: 2 },
  // 默认页（首页等）：四个全显，按此顺序排
  default: { console: 0, shop: 1, docs: 2, pricing: 3 },
};

// custom: brand — context-aware nav
export const useNavigation = (
  t,
  docsLink,
  headerNavModules,
  pathname = '/',
  shopLink = '', // custom: shop link
) => {
  const mainNavLinks = useMemo(() => {
    const modules = normalizeHeaderNavModules(headerNavModules);
    const isConsoleArea = pathname.startsWith('/console');
    const isPricingPage = pathname === '/pricing';
    const safeDocsLink = isSafeExternalUrl(docsLink) ? docsLink : ''; // custom: shop link
    const safeShopLink = isSafeExternalUrl(shopLink) ? shopLink : ''; // custom: shop link

    const orderTable = isConsoleArea
      ? ORDER_BY_PAGE.console
      : isPricingPage
        ? ORDER_BY_PAGE.pricing
        : ORDER_BY_PAGE.default;
    const getDesktopOrder = (itemKey) => orderTable[itemKey] ?? 99;

    const allLinks = [
      {
        text: t('控制台'),
        itemKey: 'console',
        to: '/console',
      },
      ...(safeDocsLink
        ? [
            {
              text: t('文档'),
              itemKey: 'docs',
              isExternal: true,
              externalLink: safeDocsLink,
            },
          ]
        : []),
      // custom: shop link — 商城外链按钮，仅在配置 URL 后显示
      ...(safeShopLink
        ? [
            {
              text: t('商城'),
              itemKey: 'shop',
              isExternal: true,
              externalLink: safeShopLink,
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
        return safeDocsLink && modules.docs;
      }
      if (link.itemKey === 'shop') {
        return Boolean(safeShopLink); // custom: shop link — URL 配置即显示，不走模块开关
      }
      if (link.itemKey === 'pricing') {
        return modules.pricing.enabled;
      }
      return modules[link.itemKey] === true;
    });

    return moduleFilteredLinks.map((link) => {
      let showOnDesktop = link.itemKey === 'console';

      if (isConsoleArea) {
        showOnDesktop = link.itemKey !== 'console';
      } else if (isPricingPage) {
        showOnDesktop = link.itemKey !== 'pricing';
      }

      return {
        ...link,
        desktopOrder: getDesktopOrder(link.itemKey),
        showOnDesktop,
      };
    });
  }, [t, docsLink, headerNavModules, pathname, shopLink]); // custom: shop link

  return {
    mainNavLinks,
  };
};
