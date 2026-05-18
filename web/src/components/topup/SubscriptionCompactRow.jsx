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
import { Tag, Tooltip } from '@douyinfe/semi-ui';
import { renderQuota } from '../../helpers';

const formatDatePart = (value) => String(value).padStart(2, '0');

const formatSubscriptionDateShort = (
  timestamp,
  currentYear = new Date().getFullYear(),
) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp * 1000);
  const monthDay = `${formatDatePart(date.getMonth() + 1)}-${formatDatePart(date.getDate())}`;
  if (date.getFullYear() !== currentYear) {
    return `${date.getFullYear()}-${monthDay}`;
  }
  return monthDay;
};

const formatSubscriptionTimeFull = (timestamp) => {
  if (!timestamp) return '-';
  const date = new Date(timestamp * 1000);
  return [
    `${date.getFullYear()}-${formatDatePart(date.getMonth() + 1)}-${formatDatePart(date.getDate())}`,
    `${formatDatePart(date.getHours())}:${formatDatePart(date.getMinutes())}:${formatDatePart(date.getSeconds())}`,
  ].join(' ');
};

const getUsagePercent = (usedAmount, totalAmount) => {
  if (
    !Number.isFinite(usedAmount) ||
    !Number.isFinite(totalAmount) ||
    totalAmount <= 0
  ) {
    return 0;
  }
  return Math.min(100, Math.max(0, (usedAmount / totalAmount) * 100));
};

const getUsagePercentLabel = (usagePercent) => {
  if (usagePercent > 0 && usagePercent < 1) return '<1';
  return String(Math.round(usagePercent));
};

const getUsageAriaValue = (usagePercent) => {
  if (!Number.isFinite(usagePercent)) return 0;
  return Number(usagePercent.toFixed(2));
};

const getSubscriptionTitle = (t, subscription, planTitleMap) => {
  const planTitle = planTitleMap.get(subscription?.plan_id) || '';
  if (planTitle) return `${planTitle} · ${t('订阅')} #${subscription?.id}`;
  return `${t('订阅')} #${subscription?.id}`;
};

export const SubscriptionCompactTime = ({ label, timestamp, currentYear }) => {
  return (
    <Tooltip content={formatSubscriptionTimeFull(timestamp)}>
      <span className='text-xs text-semi-color-text-2 tabular-nums'>
        {label} {formatSubscriptionDateShort(timestamp, currentYear)}
      </span>
    </Tooltip>
  );
};

const SubscriptionUsageLine = ({
  t,
  totalAmount,
  usedAmount,
  progressClassName,
}) => {
  if (totalAmount <= 0) {
    return (
      <div className='mt-1.5 text-xs text-semi-color-text-2'>
        {t('额度')}: {t('不限')}
      </div>
    );
  }

  const usagePercent = getUsagePercent(usedAmount, totalAmount);
  const usagePercentLabel = getUsagePercentLabel(usagePercent);

  return (
    <Tooltip
      content={`${t('原生额度')}：${usedAmount}/${totalAmount} · ${t('已用')} ${usagePercentLabel}%`}
    >
      <div className='mt-1.5 flex min-w-0 items-center gap-2'>
        <div
          className='h-1.5 min-w-16 flex-1 overflow-hidden rounded-full bg-semi-color-fill-1'
          role='progressbar'
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={getUsageAriaValue(usagePercent)}
          aria-label={`${t('已用')} ${usagePercentLabel}%`}
        >
          <div
            className={`h-full rounded-full ${progressClassName}`}
            style={{ width: `${usagePercent}%` }}
          />
        </div>
        <span className='flex-shrink-0 text-xs text-semi-color-text-2 tabular-nums'>
          {usagePercentLabel}% ({renderQuota(usedAmount)} /{' '}
          {renderQuota(totalAmount)})
        </span>
      </div>
    </Tooltip>
  );
};

// custom: subscription deduction order
const SubscriptionCompactRow = ({
  t,
  subscription,
  planTitleMap,
  statusLabel,
  statusColor,
  timeLabel,
  timeTimestamp,
  currentYear,
  meta,
  extraMeta,
  className = 'min-w-0 flex-1',
  progressClassName = 'bg-semi-color-primary transition-all duration-200',
}) => {
  const title = getSubscriptionTitle(t, subscription, planTitleMap);
  const totalAmount = Number(subscription?.amount_total || 0);
  const usedAmount = Number(subscription?.amount_used || 0);

  return (
    <div className={className}>
      <div className='flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1'>
        <Tooltip content={title}>
          <span className='min-w-0 max-w-full truncate text-sm font-medium text-semi-color-text-0'>
            {title}
          </span>
        </Tooltip>
        <Tag color={statusColor} size='small' shape='circle' type='light'>
          {statusLabel}
        </Tag>
        {meta}
        <SubscriptionCompactTime
          label={timeLabel}
          timestamp={timeTimestamp}
          currentYear={currentYear}
        />
        {extraMeta}
      </div>
      <SubscriptionUsageLine
        t={t}
        totalAmount={totalAmount}
        usedAmount={usedAmount}
        progressClassName={progressClassName}
      />
    </div>
  );
};

export default SubscriptionCompactRow;
