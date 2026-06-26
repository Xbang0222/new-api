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

import React from 'react';
import { Button, Tooltip } from '@douyinfe/semi-ui';
import { CalendarDays, Plus, RefreshCw, Search, SlidersHorizontal } from 'lucide-react';
import { useNavigate } from 'react-router-dom';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  quickRangePresets,
  activeQuickRangePreset,
  onQuickRangeSelect,
  showSearchModal,
  refresh,
  loading,
  hasApiInfoPanel,
  hasInfoPanels,
  t,
}) => {
  const navigate = useNavigate();
  const ICON_BUTTON_CLASS =
    'dashboard-icon-button !text-current focus:!bg-semi-color-fill-1 dark:focus:!bg-gray-700 !rounded-full !bg-semi-color-fill-0 dark:!bg-semi-color-fill-1 hover:!bg-semi-color-fill-1 dark:hover:!bg-semi-color-fill-2';

  const DATE_ITEM_BASE_CLASS =
    'dashboard-range-item whitespace-nowrap text-xs font-medium transition-colors duration-200';

  const activePreset = quickRangePresets.find(
    (preset) => preset.key === activeQuickRangePreset,
  );

  return (
    <div className='dashboard-header'>
      <div className='dashboard-header-copy'>
        <div className='dashboard-header-kicker'>
          <CalendarDays size={15} />
          <span>{activePreset?.label || t('自定义时间')}</span>
        </div>
        <h2
          className='dashboard-title transition-opacity duration-1000 ease-in-out'
          style={{ opacity: greetingVisible ? 1 : 0 }}
        >
          {getGreeting}
        </h2>
        <div className='dashboard-status-strip'>
          <span>{t('数据看板')}</span>
          <span>{hasApiInfoPanel ? t('API信息') : t('API信息未启用')}</span>
          <span>{hasInfoPanels ? t('服务面板') : t('服务面板未启用')}</span>
        </div>
      </div>

      <div className='dashboard-header-tools'>
        <div className='dashboard-range-scroll'>
          <div className='dashboard-range-control'>
            {quickRangePresets.map((preset) => {
              const isActive = activeQuickRangePreset === preset.key;
              return (
                <button
                  key={preset.key}
                  type='button'
                  onClick={() => onQuickRangeSelect(preset.key)}
                  className={`${DATE_ITEM_BASE_CLASS} ${
                    isActive
                      ? 'is-active text-semi-color-text-0'
                      : 'text-semi-color-text-1 hover:bg-semi-color-fill-0 hover:text-semi-color-text-0 dark:hover:bg-semi-color-fill-1'
                  }`}
                >
                  {preset.label}
                </button>
              );
            })}
          </div>
        </div>
        <div className='dashboard-header-actions'>
          <Tooltip content={t('搜索条件')}>
            <Button
              type='tertiary'
              theme='borderless'
              icon={<Search size={16} />}
              onClick={showSearchModal}
              className={ICON_BUTTON_CLASS}
              aria-label={t('搜索条件')}
            />
          </Tooltip>
          <Tooltip content={t('刷新')}>
            <Button
              type='tertiary'
              theme='borderless'
              icon={<RefreshCw size={16} />}
              onClick={refresh}
              loading={loading}
              className={ICON_BUTTON_CLASS}
              aria-label={t('刷新')}
            />
          </Tooltip>
          <Button
            theme='solid'
            type='primary'
            icon={<Plus size={16} />}
            onClick={() => navigate('/console/token')}
            className='dashboard-primary-action'
          >
            {t('创建令牌')}
          </Button>
          <div className='dashboard-filter-hint' aria-hidden='true'>
            <SlidersHorizontal size={14} />
          </div>
        </div>
      </div>
    </div>
  );
};

export default DashboardHeader;
