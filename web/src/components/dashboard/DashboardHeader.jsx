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
import { Button } from '@douyinfe/semi-ui';
import { RefreshCw, Search } from 'lucide-react';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  quickRangePresets,
  activeQuickRangePreset,
  onQuickRangeSelect,
  showSearchModal,
  refresh,
  loading,
}) => {
  const ICON_BUTTON_CLASS =
    '!p-1 !text-current focus:!bg-semi-color-fill-1 dark:focus:!bg-gray-700 !rounded-full !bg-semi-color-fill-0 dark:!bg-semi-color-fill-1 hover:!bg-semi-color-fill-1 dark:hover:!bg-semi-color-fill-2';

  const DATE_ITEM_BASE_CLASS =
    'h-7 min-w-[56px] px-2 rounded-full whitespace-nowrap text-xs font-medium transition-colors duration-200';

  return (
    <div className='mb-4 flex flex-col gap-2 lg:flex-row lg:items-center'>
      <h2
        className='text-xl md:text-[22px] font-semibold text-semi-color-text-0 transition-opacity duration-1000 ease-in-out'
        style={{ opacity: greetingVisible ? 1 : 0 }}
      >
        {getGreeting}
      </h2>
      <div className='flex min-w-0 items-center gap-2 lg:ml-4 lg:flex-1'>
        <div className='min-w-0 flex-1 overflow-x-auto'>
          <div className='inline-flex h-8 min-w-max items-center rounded-full bg-semi-color-fill-0 p-0.5 dark:bg-semi-color-fill-1'>
            {quickRangePresets.map((preset) => {
              const isActive = activeQuickRangePreset === preset.key;
              return (
                <button
                  key={preset.key}
                  type='button'
                  onClick={() => onQuickRangeSelect(preset.key)}
                  className={`${DATE_ITEM_BASE_CLASS} ${
                    isActive
                      ? 'bg-semi-color-fill-0 text-semi-color-text-0 dark:bg-semi-color-fill-1'
                      : 'text-semi-color-text-1 hover:bg-semi-color-fill-0 hover:text-semi-color-text-0 dark:hover:bg-semi-color-fill-1'
                  }`}
                >
                  {preset.label}
                </button>
              );
            })}
          </div>
        </div>
        <div className='flex shrink-0 items-center gap-1.5'>
          <Button
            type='tertiary'
            theme='borderless'
            icon={<Search size={15} />}
            onClick={showSearchModal}
            className={ICON_BUTTON_CLASS}
          />
          <Button
            type='tertiary'
            theme='borderless'
            icon={<RefreshCw size={15} />}
            onClick={refresh}
            loading={loading}
            className={ICON_BUTTON_CLASS}
          />
        </div>
      </div>
    </div>
  );
};

export default DashboardHeader;
