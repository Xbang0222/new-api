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

// custom: subscription cycle purchase limit
// Frontend mirror of backend `calcPurchaseWindowStart` so the
// "已达到购买上限" hint counts purchases within the same rolling
// window as the server-side enforcement.

const SECONDS_PER_DAY = 86400;
const SECONDS_PER_HOUR = 3600;
const SUBSCRIPTION_NEAR_EXHAUSTED_THRESHOLD = 0.99;

/**
 * Subtract `value` months from a Unix timestamp using calendar arithmetic
 * (matches Go's time.AddDate behavior — normalizes day-of-month overflow,
 * e.g., March 31 minus 1 month → March 3).
 */
function subtractMonthsFromUnix(unixSec, value) {
  const date = new Date(unixSec * 1000);
  const targetMonth = date.getUTCMonth() - value;
  const result = new Date(
    Date.UTC(
      date.getUTCFullYear(),
      targetMonth,
      date.getUTCDate(),
      date.getUTCHours(),
      date.getUTCMinutes(),
      date.getUTCSeconds(),
      date.getUTCMilliseconds(),
    ),
  );
  return Math.floor(result.getTime() / 1000);
}

function subtractYearsFromUnix(unixSec, value) {
  return subtractMonthsFromUnix(unixSec, value * 12);
}

/**
 * Compute the rolling window start (Unix seconds) for the given plan,
 * relative to `nowSec`. Returns 0 when the plan has no valid duration.
 */
export function computePurchaseWindowStart(nowSec, plan) {
  if (!plan) return 0;
  const unit = plan.duration_unit;
  const value = Number(plan.duration_value || 0);
  switch (unit) {
    case 'year':
      if (value <= 0) return 0;
      return subtractYearsFromUnix(nowSec, value);
    case 'month':
      if (value <= 0) return 0;
      return subtractMonthsFromUnix(nowSec, value);
    case 'day':
      if (value <= 0) return 0;
      return nowSec - value * SECONDS_PER_DAY;
    case 'hour':
      if (value <= 0) return 0;
      return nowSec - value * SECONDS_PER_HOUR;
    case 'custom': {
      const seconds = Number(plan.custom_seconds || 0);
      if (seconds <= 0) return 0;
      return nowSec - seconds;
    }
    default:
      return 0;
  }
}

// custom: subscription deduction order
export function isSubscriptionNearExhausted(
  subscription,
  threshold = SUBSCRIPTION_NEAR_EXHAUSTED_THRESHOLD,
) {
  const total = Number(subscription?.amount_total || 0);
  if (total <= 0) return false;
  const used = Number(subscription?.amount_used || 0);
  return used / total >= threshold;
}
