// custom: subscription cycle purchase limit
// Frontend mirror of backend `calcPurchaseWindowStart` so the
// "已达到购买上限" hint counts purchases within the same rolling
// window as the server-side enforcement.

const SECONDS_PER_DAY = 86400;
const SECONDS_PER_HOUR = 3600;

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
