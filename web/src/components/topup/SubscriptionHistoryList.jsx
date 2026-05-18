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
import { isSubscriptionNearExhausted } from '../../helpers/subscription';
import SubscriptionCompactRow from './SubscriptionCompactRow';

const getHistoryStatus = (t, subscription, now) => {
  if (subscription?.status === 'cancelled') {
    return { label: t('已作废'), color: 'grey' };
  }
  if ((subscription?.end_time || 0) < now) {
    return { label: t('已过期'), color: 'grey' };
  }
  if (isSubscriptionNearExhausted(subscription)) {
    return { label: t('已用完'), color: 'orange' };
  }
  return { label: t('已过期'), color: 'grey' };
};

// custom: subscription deduction order
const SubscriptionHistoryList = ({ t, subscriptions, planTitleMap }) => {
  const now = Date.now() / 1000;
  const currentYear = new Date().getFullYear();

  return (
    <div className='max-h-[18rem] overflow-y-auto rounded-lg border border-semi-color-border bg-semi-color-fill-0 semi-table-body'>
      {subscriptions.map((sub, subIndex) => {
        const subscription = sub.subscription;
        const status = getHistoryStatus(t, subscription, now);

        return (
          <div
            key={subscription?.id || subIndex}
            className='border-b border-semi-color-border bg-semi-color-fill-0 px-3 py-2 opacity-75 last:border-b-0'
          >
            <SubscriptionCompactRow
              t={t}
              subscription={subscription}
              planTitleMap={planTitleMap}
              statusLabel={status.label}
              statusColor={status.color}
              timeLabel={t('至')}
              timeTimestamp={subscription?.end_time}
              currentYear={currentYear}
              className='min-w-0'
              progressClassName='bg-semi-color-text-3'
            />
          </div>
        );
      })}
    </div>
  );
};

export default SubscriptionHistoryList;
