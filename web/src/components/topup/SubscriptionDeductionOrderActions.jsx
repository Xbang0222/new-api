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
import { ArrowDown, ArrowUp } from 'lucide-react';

// custom: subscription deduction order
const SubscriptionDeductionOrderActions = ({
  t,
  remainDays,
  canMoveUp,
  canMoveDown,
  savingOrder,
  onMoveUp,
  onMoveDown,
}) => {
  return (
    <div className='flex items-center gap-1 flex-shrink-0'>
      <span className='text-gray-500 mr-1 whitespace-nowrap'>
        {t('剩余')} {remainDays} {t('天')}
      </span>
      <Tooltip content={t('上移')}>
        <Button
          aria-label={t('上移')}
          size='small'
          theme='borderless'
          type='tertiary'
          icon={<ArrowUp size={12} />}
          disabled={!canMoveUp || savingOrder}
          onClick={onMoveUp}
        />
      </Tooltip>
      <Tooltip content={t('下移')}>
        <Button
          aria-label={t('下移')}
          size='small'
          theme='borderless'
          type='tertiary'
          icon={<ArrowDown size={12} />}
          disabled={!canMoveDown || savingOrder}
          onClick={onMoveDown}
        />
      </Tooltip>
    </div>
  );
};

export default SubscriptionDeductionOrderActions;
