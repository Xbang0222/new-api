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
import { Badge, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';
import { renderQuota } from '../../helpers';
import SubscriptionDeductionOrderActions from './SubscriptionDeductionOrderActions';

const { Text } = Typography;

const formatSubscriptionTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const getSubscriptionStatus = (subscription, now) => {
  const isExpired = (subscription?.end_time || 0) < now;
  const isCancelled = subscription?.status === 'cancelled';
  const isActive = subscription?.status === 'active' && !isExpired;
  return { isActive, isCancelled };
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

const SubscriptionStatusTag = ({ t, isActive, isCancelled }) => {
  return (
    <Tag
      color={isActive ? 'green' : 'grey'}
      size='small'
      shape='circle'
      type='light'
      prefixIcon={isActive ? <Badge dot type='success' /> : undefined}
    >
      {isActive ? t('生效') : isCancelled ? t('已作废') : t('已过期')}
    </Tag>
  );
};

const DeductionOrderMarker = ({
  t,
  orderNumber,
  isActive,
  showOrderControls,
}) => {
  if (isActive && showOrderControls) {
    return (
      <span
        className='mt-0.5 inline-flex h-6 min-w-9 flex-shrink-0 items-center justify-center rounded-md border border-semi-color-border bg-semi-color-fill-0 px-2 text-[11px] font-medium leading-none text-semi-color-text-0 tabular-nums'
        aria-label={`${t('扣费按列表顺序')} #${orderNumber}`}
        title={`#${orderNumber}`}
      >
        #{orderNumber}
      </span>
    );
  }

  if (isActive) {
    return null;
  }

  return (
    <span
      className='mt-0.5 hidden min-w-9 flex-shrink-0 text-center text-xs leading-6 text-semi-color-text-3 sm:block'
      aria-hidden='true'
    >
      -
    </span>
  );
};

const SubscriptionQuotaLine = ({
  t,
  totalAmount,
  usedAmount,
  remainAmount,
}) => {
  if (totalAmount <= 0) {
    return (
      <div className='mt-2 text-xs text-semi-color-text-2'>
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
      <div className='mt-2'>
        <div className='flex items-center justify-between gap-3 text-xs text-semi-color-text-2'>
          <span className='truncate'>
            {t('已用')} {renderQuota(usedAmount)} / {renderQuota(totalAmount)}
          </span>
          <span className='flex-shrink-0 tabular-nums'>
            {usagePercentLabel}%
          </span>
        </div>
        <div
          className='mt-1.5 h-1 overflow-hidden rounded-full bg-semi-color-fill-1'
          role='progressbar'
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={getUsageAriaValue(usagePercent)}
          aria-label={`${t('已用')} ${usagePercentLabel}%`}
        >
          <div
            className='h-full rounded-full bg-semi-color-primary transition-all duration-200'
            style={{ width: `${usagePercent}%` }}
          />
        </div>
        <div className='mt-1 text-[11px] leading-none text-semi-color-text-3'>
          {t('剩余')} {renderQuota(remainAmount)}
        </div>
      </div>
    </Tooltip>
  );
};

// custom: subscription deduction order
const SubscriptionDeductionOrderList = ({
  t,
  subscriptions,
  orderedActiveCount,
  showOrderControls,
  planTitleMap,
  savingOrder,
  onMove,
}) => {
  const now = Date.now() / 1000;

  return (
    <div className='max-h-[22rem] overflow-y-auto rounded-lg border border-semi-color-border bg-semi-color-fill-0 semi-table-body'>
      {subscriptions.map((sub, subIndex) => {
        const subscription = sub.subscription;
        const totalAmount = Number(subscription?.amount_total || 0);
        const usedAmount = Number(subscription?.amount_used || 0);
        const remainAmount =
          totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0;
        const planTitle = planTitleMap.get(subscription?.plan_id) || '';
        const remainDays = Math.max(
          0,
          Math.ceil(((subscription?.end_time || 0) - now) / 86400),
        );
        const { isActive, isCancelled } = getSubscriptionStatus(
          subscription,
          now,
        );
        const canMoveUp = showOrderControls && isActive && subIndex > 0;
        const canMoveDown =
          showOrderControls && isActive && subIndex < orderedActiveCount - 1;
        const orderNumber = isActive ? subIndex + 1 : 0;
        const endTimeLabel = isActive
          ? t('至')
          : isCancelled
            ? t('作废于')
            : t('过期于');
        const title = planTitle
          ? `${planTitle} · ${t('订阅')} #${subscription?.id}`
          : `${t('订阅')} #${subscription?.id}`;

        return (
          <div
            key={subscription?.id || subIndex}
            className={`border-b border-semi-color-border p-3 transition-colors last:border-b-0 ${
              isActive
                ? 'bg-semi-color-bg-0 hover:bg-semi-color-fill-0'
                : 'bg-semi-color-fill-0 opacity-75'
            }`}
          >
            <div className='flex items-start gap-3'>
              <div className='flex min-w-0 flex-1 gap-2.5'>
                <DeductionOrderMarker
                  t={t}
                  orderNumber={orderNumber}
                  isActive={isActive}
                  showOrderControls={showOrderControls}
                />
                <div className='min-w-0 flex-1'>
                  <div className='flex min-w-0 flex-wrap items-center gap-2'>
                    <Text
                      strong
                      className='min-w-0 max-w-full'
                      ellipsis={{ showTooltip: true }}
                      style={{ display: 'block' }}
                    >
                      {title}
                    </Text>
                    <SubscriptionStatusTag
                      t={t}
                      isActive={isActive}
                      isCancelled={isCancelled}
                    />
                  </div>
                  <div className='mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-semi-color-text-2'>
                    <span>
                      {endTimeLabel}{' '}
                      {formatSubscriptionTime(subscription?.end_time)}
                    </span>
                    {isActive && (
                      <span className='tabular-nums'>
                        {t('剩余')} {remainDays} {t('天')}
                      </span>
                    )}
                    {isActive && subscription?.next_reset_time > 0 && (
                      <span>
                        {t('下一次重置')}:{' '}
                        {formatSubscriptionTime(subscription.next_reset_time)}
                      </span>
                    )}
                  </div>
                  <SubscriptionQuotaLine
                    t={t}
                    totalAmount={totalAmount}
                    usedAmount={usedAmount}
                    remainAmount={remainAmount}
                  />
                </div>
              </div>

              {isActive && showOrderControls && (
                <SubscriptionDeductionOrderActions
                  t={t}
                  canMoveUp={canMoveUp}
                  canMoveDown={canMoveDown}
                  savingOrder={savingOrder}
                  onMoveUp={() => onMove(subIndex, -1)}
                  onMoveDown={() => onMove(subIndex, 1)}
                />
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
};

export default SubscriptionDeductionOrderList;
