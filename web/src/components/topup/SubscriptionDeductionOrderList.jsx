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

import React, { useState } from 'react';
import { GripVertical } from 'lucide-react';
import SubscriptionCompactRow, {
  SubscriptionCompactTime,
} from './SubscriptionCompactRow';

// custom: subscription deduction order
const SubscriptionDeductionOrderList = ({
  t,
  subscriptions,
  showOrderControls,
  planTitleMap,
  savingOrder,
  onReorder,
}) => {
  const [draggedId, setDraggedId] = useState('');
  const [dragOverId, setDragOverId] = useState('');
  const [dragOverPosition, setDragOverPosition] = useState('before');
  const now = Date.now() / 1000;
  const currentYear = new Date().getFullYear();

  const resetDragState = () => {
    setDraggedId('');
    setDragOverId('');
    setDragOverPosition('before');
  };

  const handleDragStart = (event, subscriptionId) => {
    setDraggedId(subscriptionId);
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', subscriptionId);
  };

  const handleDragOver = (event, subscriptionId) => {
    event.preventDefault();
    if (!draggedId) return;
    if (draggedId === subscriptionId) {
      setDragOverId('');
      return;
    }
    const rect = event.currentTarget.getBoundingClientRect();
    const position =
      event.clientY - rect.top > rect.height / 2 ? 'after' : 'before';
    setDragOverId(subscriptionId);
    setDragOverPosition(position);
    event.dataTransfer.dropEffect = 'move';
  };

  const handleContainerDragLeave = (event) => {
    // contains(null) === false 在所有浏览器里都成立,
    // 包括 Firefox 拖出 window 时 relatedTarget=null 的边角
    if (!event.currentTarget.contains(event.relatedTarget)) {
      setDragOverId('');
      setDragOverPosition('before');
    }
  };

  const handleDrop = (event, subscriptionId) => {
    event.preventDefault();
    const sourceId = draggedId || event.dataTransfer.getData('text/plain');
    const position =
      dragOverId === subscriptionId ? dragOverPosition : 'before';
    if (sourceId && sourceId !== subscriptionId) {
      onReorder?.(sourceId, subscriptionId, position);
    }
    resetDragState();
  };

  return (
    <div
      className='max-h-[22rem] overflow-y-auto rounded-lg border border-semi-color-border bg-semi-color-fill-0 semi-table-body'
      onDragLeave={handleContainerDragLeave}
    >
      {subscriptions.map((sub, subIndex) => {
        const subscription = sub.subscription;
        const subscriptionId = String(subscription?.id || '');
        const remainDays = Math.max(
          0,
          Math.ceil(((subscription?.end_time || 0) - now) / 86400),
        );
        const showDragHandle = showOrderControls;
        const canDrag = showDragHandle && !savingOrder;
        const isDragging = draggedId === subscriptionId;
        const isDropTarget =
          dragOverId === subscriptionId &&
          draggedId &&
          draggedId !== subscriptionId;

        return (
          <div
            key={subscription?.id || subIndex}
            draggable={canDrag}
            onDragStart={(event) => handleDragStart(event, subscriptionId)}
            onDragOver={(event) => handleDragOver(event, subscriptionId)}
            onDrop={(event) => handleDrop(event, subscriptionId)}
            onDragEnd={resetDragState}
            className={`group relative border-b border-semi-color-border bg-semi-color-bg-0 px-3 py-2 transition-colors last:border-b-0 hover:bg-semi-color-fill-0 ${
              canDrag ? 'cursor-grab active:cursor-grabbing' : ''
            } ${isDragging ? 'opacity-50' : ''}`}
          >
            {isDropTarget && (
              <div
                className={`absolute left-0 right-0 z-10 h-0.5 bg-semi-color-primary ${
                  dragOverPosition === 'after' ? 'bottom-0' : 'top-0'
                }`}
              />
            )}
            <div className='flex min-w-0 items-start gap-2.5'>
              {showDragHandle && (
                <span
                  className={`mt-0.5 inline-flex h-6 flex-shrink-0 items-center gap-1 rounded-md border border-semi-color-border bg-semi-color-fill-0 px-1.5 text-[11px] font-medium leading-none text-semi-color-text-2 transition-colors ${
                    canDrag
                      ? 'group-hover:border-semi-color-primary group-hover:text-semi-color-primary'
                      : 'opacity-60'
                  }`}
                  aria-label={`${t('拖动调整扣费顺序')} #${subIndex + 1}`}
                  title={`#${subIndex + 1}`}
                >
                  <GripVertical size={14} />
                  <span className='tabular-nums'>#{subIndex + 1}</span>
                </span>
              )}
              <SubscriptionCompactRow
                t={t}
                subscription={subscription}
                planTitleMap={planTitleMap}
                statusLabel={t('生效')}
                statusColor='green'
                timeLabel={t('至')}
                timeTimestamp={subscription?.end_time}
                currentYear={currentYear}
                meta={
                  <span className='text-xs text-semi-color-text-2 tabular-nums'>
                    {t('剩余')} {remainDays} {t('天')}
                  </span>
                }
                extraMeta={
                  subscription?.next_reset_time > 0 ? (
                    <SubscriptionCompactTime
                      label={`${t('下一次重置')}:`}
                      timestamp={subscription.next_reset_time}
                      currentYear={currentYear}
                    />
                  ) : null
                }
              />
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default SubscriptionDeductionOrderList;
