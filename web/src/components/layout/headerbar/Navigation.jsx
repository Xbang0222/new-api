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

import React, { useMemo } from 'react';
import { NavLink } from 'react-router-dom';
import { Grid2x2, FileText, LayoutGrid } from 'lucide-react';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
}) => {
  const displayedNavLinks = useMemo(() => {
    if (isMobile) {
      return mainNavLinks;
    }
    return mainNavLinks.filter((link) => link.showOnDesktop !== false);
  }, [isMobile, mainNavLinks]);

  const getTargetPath = (link) => {
    if (link.itemKey === 'console' && !userState.user) {
      return '/login';
    }
    if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
      return '/login';
    }
    return link.to;
  };

  const getLinkIcon = (itemKey) => {
    if (itemKey === 'console') {
      return <Grid2x2 size={16} />;
    }
    if (itemKey === 'docs') {
      return <FileText size={16} />;
    }
    if (itemKey === 'pricing') {
      return <LayoutGrid size={16} />;
    }
    return null;
  };

  const getDesktopLinkClasses = (isActive) => {
    const baseClasses =
      'flex-shrink-0 inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm font-semibold transition-colors duration-200';
    if (isActive) {
      return `${baseClasses} bg-semi-color-fill-0 dark:bg-semi-color-fill-1 text-semi-color-text-0`;
    }
    return `${baseClasses} text-semi-color-text-1 hover:bg-semi-color-fill-0 dark:hover:bg-semi-color-fill-1 hover:text-semi-color-text-0`;
  };

  const mobileLinkClasses =
    'flex-shrink-0 flex items-center gap-1 font-semibold rounded-md p-1 transition-all duration-200 ease-in-out hover:text-semi-color-primary';

  const renderDesktopNavLink = (link) => {
    const linkContent = (
      <>
        {getLinkIcon(link.itemKey)}
        <span>{link.text}</span>
      </>
    );

    if (link.isExternal) {
      return (
        <a
          key={link.itemKey}
          href={link.externalLink}
          target='_blank'
          rel='noopener noreferrer'
          className={getDesktopLinkClasses(false)}
        >
          {linkContent}
        </a>
      );
    }

    return (
      <NavLink
        key={link.itemKey}
        to={getTargetPath(link)}
        className={({ isActive }) => getDesktopLinkClasses(isActive)}
      >
        {linkContent}
      </NavLink>
    );
  };

  const renderMobileNavLink = (link) => {
    if (link.isExternal) {
      return (
        <a
          key={link.itemKey}
          href={link.externalLink}
          target='_blank'
          rel='noopener noreferrer'
          className={mobileLinkClasses}
        >
          <span>{link.text}</span>
        </a>
      );
    }

    return (
      <NavLink key={link.itemKey} to={getTargetPath(link)} className={mobileLinkClasses}>
        <span>{link.text}</span>
      </NavLink>
    );
  };

  const navClassName = isMobile
    ? 'flex flex-1 items-center gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto whitespace-nowrap scrollbar-hide'
    : 'flex items-center gap-1 lg:gap-2';

  return (
    <nav className={navClassName}>
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={displayedNavLinks.length || 3}
        width={isMobile ? 60 : 96}
        height={16}
        isMobile={isMobile}
      >
        {displayedNavLinks.map((link) =>
          isMobile ? renderMobileNavLink(link) : renderDesktopNavLink(link),
        )}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
