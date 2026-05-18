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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  Collapse,
  Divider,
  Select,
  Skeleton,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess, renderQuota } from '../../helpers';
import { getCurrencyConfig } from '../../helpers/render';
import { RefreshCw, Sparkles } from 'lucide-react';
import SubscriptionPurchaseModal from './modals/SubscriptionPurchaseModal';
// custom: subscription deduction order
import SubscriptionDeductionOrderList from './SubscriptionDeductionOrderList';
import SubscriptionHistoryList from './SubscriptionHistoryList';
import {
  formatSubscriptionDuration,
  formatSubscriptionResetPeriod,
} from '../../helpers/subscriptionFormat';
// custom: subscription cycle purchase limit
import {
  computePurchaseWindowStart,
  isSubscriptionNearExhausted,
} from '../../helpers/subscription';

const { Text } = Typography;

// 过滤易支付方式
function getEpayMethods(payMethods = []) {
  return (payMethods || []).filter(
    (m) => m?.type && m.type !== 'stripe' && m.type !== 'creem',
  );
}

// 提交易支付表单
function submitEpayForm({ url, params }) {
  const form = document.createElement('form');
  form.action = url;
  form.method = 'POST';
  const isSafari =
    navigator.userAgent.indexOf('Safari') > -1 &&
    navigator.userAgent.indexOf('Chrome') < 1;
  if (!isSafari) form.target = '_blank';
  Object.keys(params || {}).forEach((key) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = key;
    input.value = params[key];
    form.appendChild(input);
  });
  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

// custom: subscription deduction order
const getSubscriptionId = (sub) => sub?.subscription?.id;

// custom: subscription deduction order
const hasSameSubscriptionOrder = (left, right) => {
  if (left.length !== right.length) return false;
  return left.every(
    (sub, index) => getSubscriptionId(sub) === getSubscriptionId(right[index]),
  );
};

// custom: subscription deduction order
const reorderSubscriptionList = (
  subscriptions,
  sourceId,
  targetId,
  position = 'before',
) => {
  if (!sourceId || !targetId || sourceId === targetId) return subscriptions;
  const sourceIndex = subscriptions.findIndex(
    (sub) => String(getSubscriptionId(sub)) === String(sourceId),
  );
  const targetIndex = subscriptions.findIndex(
    (sub) => String(getSubscriptionId(sub)) === String(targetId),
  );
  if (sourceIndex < 0 || targetIndex < 0) return subscriptions;

  const next = [...subscriptions];
  const [moved] = next.splice(sourceIndex, 1);
  const newTargetIndex = next.findIndex(
    (sub) => String(getSubscriptionId(sub)) === String(targetId),
  );
  if (newTargetIndex < 0) return subscriptions;
  const insertIndex =
    position === 'after' ? newTargetIndex + 1 : newTargetIndex;
  next.splice(insertIndex, 0, moved);

  return hasSameSubscriptionOrder(subscriptions, next) ? subscriptions : next;
};

// custom: subscription deduction order
const getDeductionOrderSubscriptionIds = (subscriptions) => {
  const visibleIds = [];
  const nearExhaustedIds = [];
  subscriptions.forEach((sub) => {
    const id = getSubscriptionId(sub);
    if (!id) return;
    if (isSubscriptionNearExhausted(sub?.subscription)) {
      nearExhaustedIds.push(id);
      return;
    }
    visibleIds.push(id);
  });
  return [...visibleIds, ...nearExhaustedIds];
};

const SubscriptionPlansCard = ({
  t,
  loading = false,
  plans = [],
  payMethods = [],
  enableOnlineTopUp = false,
  enableStripeTopUp = false,
  enableCreemTopUp = false,
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
  withCard = true,
  onRefreshUserQuota, // custom: wallet subscription — refresh balance after wallet payment
}) => {
  const [open, setOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [paying, setPaying] = useState(false);
  const [selectedEpayMethod, setSelectedEpayMethod] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  // custom: subscription deduction order
  const [orderedActiveSubscriptions, setOrderedActiveSubscriptions] = useState(
    activeSubscriptions || [],
  );
  const [savingOrder, setSavingOrder] = useState(false);

  const epayMethods = useMemo(() => getEpayMethods(payMethods), [payMethods]);

  // custom: subscription deduction order
  useEffect(() => {
    if (savingOrder) return;
    setOrderedActiveSubscriptions(activeSubscriptions || []);
  }, [activeSubscriptions, savingOrder]);

  // custom: wallet subscription — add wallet as first option in the epay dropdown
  const allPayMethods = useMemo(() => {
    const walletOpt = {
      type: 'wallet',
      name: t('钱包支付'),
    };
    return [walletOpt, ...epayMethods];
  }, [epayMethods, t]);

  const openBuy = (p) => {
    setSelectedPlan(p);
    setSelectedEpayMethod(allPayMethods?.[0]?.type || '');
    setOpen(true);
  };

  const closeBuy = () => {
    setOpen(false);
    setSelectedPlan(null);
    setPaying(false);
  };

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await reloadSubscriptionSelf?.();
    } finally {
      setRefreshing(false);
    }
  };

  // custom: subscription deduction order
  const refreshSubscriptionSelfSilently = async () => {
    try {
      await reloadSubscriptionSelf?.();
    } catch {
      // custom: subscription deduction order; save result has already been shown
    }
  };

  const payStripe = async () => {
    if (!selectedPlan?.plan?.stripe_price_id) {
      showError(t('该套餐未配置 Stripe'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/stripe/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.pay_link, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payCreem = async () => {
    if (!selectedPlan?.plan?.creem_product_id) {
      showError(t('该套餐未配置 Creem'));
      return;
    }
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/creem/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        window.open(res.data.data?.checkout_url, '_blank');
        showSuccess(t('已打开支付页面'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  const payEpay = async () => {
    if (!selectedEpayMethod) {
      showError(t('请选择支付方式'));
      return;
    }
    // custom: wallet subscription — intercept wallet selection
    if (selectedEpayMethod === 'wallet') return payWallet();
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/epay/pay', {
        plan_id: selectedPlan.plan.id,
        payment_method: selectedEpayMethod,
      });
      if (res.data?.message === 'success') {
        submitEpayForm({ url: res.data.url, params: res.data.data });
        showSuccess(t('已发起支付'));
        closeBuy();
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  // custom: wallet subscription payment
  const payWallet = async () => {
    setPaying(true);
    try {
      const res = await API.post('/api/subscription/wallet/pay', {
        plan_id: selectedPlan.plan.id,
      });
      if (res.data?.message === 'success') {
        showSuccess(t('订阅购买成功'));
        closeBuy();
        await reloadSubscriptionSelf?.();
        await onRefreshUserQuota?.(); // custom: wallet subscription — refresh balance
      } else {
        const errorMsg =
          typeof res.data?.data === 'string'
            ? res.data.data
            : res.data?.message || t('支付失败');
        showError(errorMsg);
      }
    } catch (e) {
      showError(t('支付请求失败'));
    } finally {
      setPaying(false);
    }
  };

  // custom: subscription deduction order
  const activeVisibleSubscriptions = useMemo(
    () =>
      orderedActiveSubscriptions.filter(
        (sub) => !isSubscriptionNearExhausted(sub?.subscription),
      ),
    [orderedActiveSubscriptions],
  );

  // custom: subscription deduction order
  const nearExhaustedActiveSubscriptions = useMemo(
    () =>
      orderedActiveSubscriptions.filter((sub) =>
        isSubscriptionNearExhausted(sub?.subscription),
      ),
    [orderedActiveSubscriptions],
  );

  // 当前订阅信息 - 支持多个订阅
  const hasActiveSubscription = activeVisibleSubscriptions.length > 0;
  const hasAnySubscription =
    allSubscriptions.length > 0 || orderedActiveSubscriptions.length > 0;
  // custom: subscription deduction order
  const showSubscriptionOrderControls = activeVisibleSubscriptions.length > 1;
  const disableSubscriptionPreference = !hasActiveSubscription;
  const isSubscriptionPreference =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only';
  const displayBillingPreference =
    disableSubscriptionPreference && isSubscriptionPreference
      ? 'wallet_first'
      : billingPreference;
  const subscriptionPreferenceLabel =
    billingPreference === 'subscription_only' ? t('仅用订阅') : t('优先订阅');

  // custom: subscription cycle purchase limit
  // Originally counted every UserSubscription lifetime per plan. We now
  // restrict counts to the rolling time window that matches the plan's
  // own duration (mirrors backend `countPurchasesInWindow`), so the
  // "已达到购买上限 (count/limit)" hint stays consistent with enforcement.
  const planPurchaseCountMap = useMemo(() => {
    const map = new Map();
    const nowSec = Math.floor(Date.now() / 1000);
    const windowStartByPlan = new Map();
    (plans || []).forEach((p) => {
      const plan = p?.plan;
      if (!plan?.id) return;
      windowStartByPlan.set(plan.id, computePurchaseWindowStart(nowSec, plan));
    });
    (allSubscriptions || []).forEach((sub) => {
      const s = sub?.subscription;
      const planId = s?.plan_id;
      if (!planId) return;
      const windowStart = windowStartByPlan.get(planId) || 0;
      const createdAt = Number(s?.created_at || 0);
      if (windowStart > 0 && createdAt > 0 && createdAt < windowStart) return;
      map.set(planId, (map.get(planId) || 0) + 1);
    });
    return map;
  }, [allSubscriptions, plans]);

  const planTitleMap = useMemo(() => {
    const map = new Map();
    (plans || []).forEach((p) => {
      const plan = p?.plan;
      if (!plan?.id) return;
      map.set(plan.id, plan.title || '');
    });
    return map;
  }, [plans]);

  // custom: subscription deduction order
  const activeSubscriptionIdSet = useMemo(() => {
    const ids = new Set();
    (activeSubscriptions || []).forEach((sub) => {
      const id = sub?.subscription?.id;
      if (id) ids.add(id);
    });
    return ids;
  }, [activeSubscriptions]);

  // custom: subscription deduction order
  const pendingOrderRef = useRef(null);
  const inFlightRef = useRef(false);

  // custom: subscription deduction order
  // NIT 1c: 过滤掉拖动期间已经被父组件移出 active 列表的订阅 ID,
  // 不依赖后端 WHERE active 兜底
  const sanitizePendingNext = (next) =>
    next.filter((sub) => {
      const id = sub?.subscription?.id;
      return id && activeSubscriptionIdSet.has(id);
    });

  // custom: subscription deduction order
  // 串行队列：同一时刻最多 1 个 PUT 在飞；拖动期间产生的多次 reorder
  // 只发送最新一次；reloadSubscriptionSelf 只在队列彻底清空后调一次。
  const flushSaveOrder = async () => {
    if (inFlightRef.current) return;
    inFlightRef.current = true;
    try {
      while (pendingOrderRef.current) {
        const raw = pendingOrderRef.current;
        pendingOrderRef.current = null;
        const next = sanitizePendingNext(raw);
        const subscriptionIds = getDeductionOrderSubscriptionIds(next);
        try {
          const res = await API.put('/api/subscription/self/order', {
            subscription_ids: subscriptionIds,
          });
          if (res.data?.success) {
            showSuccess(t('排序已保存'));
          } else {
            showError(res.data?.message || t('保存失败，请刷新后重试'));
          }
        } catch (e) {
          showError(
            e?.response?.data?.message || t('保存失败，请刷新后重试'),
          );
        }
      }
      await refreshSubscriptionSelfSilently();
    } finally {
      inFlightRef.current = false;
      setSavingOrder(false);
    }
  };

  // custom: subscription deduction order
  const reorderSubscriptions = (sourceId, targetId, position) => {
    const next = reorderSubscriptionList(
      orderedActiveSubscriptions,
      sourceId,
      targetId,
      position,
    );
    if (next === orderedActiveSubscriptions) return;
    // BLOCKING 1b: setSavingOrder 必须与 setOrderedActiveSubscriptions 在同一 batch,
    // 防止父组件同 tick 的 activeSubscriptions 更新触发 useEffect 用旧 props 覆盖
    setSavingOrder(true);
    setOrderedActiveSubscriptions(next);
    pendingOrderRef.current = next;
    void flushSaveOrder();
  };

  // custom: subscription deduction order
  const inactiveSubscriptions = useMemo(() => {
    return (allSubscriptions || []).filter((sub) => {
      const id = sub?.subscription?.id;
      return !id || !activeSubscriptionIdSet.has(id);
    });
  }, [allSubscriptions, activeSubscriptionIdSet]);

  // custom: subscription deduction order
  const historySubscriptions = useMemo(
    () => [...inactiveSubscriptions, ...nearExhaustedActiveSubscriptions],
    [inactiveSubscriptions, nearExhaustedActiveSubscriptions],
  );

  const getPlanPurchaseCount = (planId) =>
    planPurchaseCountMap.get(planId) || 0;

  const cardContent = (
    <>
      {/* 卡片头部 */}
      {loading ? (
        <div className='space-y-4'>
          {/* 我的订阅骨架屏 */}
          <Card className='!rounded-xl w-full' bodyStyle={{ padding: '12px' }}>
            <div className='flex items-center justify-between mb-3'>
              <Skeleton.Title active style={{ width: 100, height: 20 }} />
              <Skeleton.Button active style={{ width: 24, height: 24 }} />
            </div>
            <div className='space-y-2'>
              <Skeleton.Paragraph active rows={2} />
            </div>
          </Card>
          {/* 套餐列表骨架屏 */}
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 gap-5 w-full px-1'>
            {[1, 2, 3].map((i) => (
              <Card
                key={i}
                className='!rounded-xl w-full h-full'
                bodyStyle={{ padding: 16 }}
              >
                <Skeleton.Title
                  active
                  style={{ width: '60%', height: 24, marginBottom: 8 }}
                />
                <Skeleton.Paragraph
                  active
                  rows={1}
                  style={{ marginBottom: 12 }}
                />
                <div className='text-center py-4'>
                  <Skeleton.Title
                    active
                    style={{ width: '40%', height: 32, margin: '0 auto' }}
                  />
                </div>
                <Skeleton.Paragraph active rows={3} style={{ marginTop: 12 }} />
                <Skeleton.Button
                  active
                  block
                  style={{ marginTop: 16, height: 32 }}
                />
              </Card>
            ))}
          </div>
        </div>
      ) : (
        <Space vertical style={{ width: '100%' }} spacing={8}>
          {/* 当前订阅状态 */}
          <div className='w-full rounded-xl border border-semi-color-border bg-semi-color-bg-0 p-4'>
            <div className='flex flex-col gap-3 md:flex-row md:items-start md:justify-between'>
              <div className='min-w-0 flex-1'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Text strong>{t('我的订阅')}</Text>
                  {hasActiveSubscription ? (
                    <Tag
                      color='green'
                      size='small'
                      shape='circle'
                      type='light'
                      prefixIcon={<Badge dot type='success' />}
                    >
                      {activeVisibleSubscriptions.length} {t('个生效中')}
                    </Tag>
                  ) : (
                    <Tag color='grey' size='small' shape='circle' type='light'>
                      {t('无生效')}
                    </Tag>
                  )}
                  {historySubscriptions.length > 0 && (
                    <Tag color='grey' size='small' shape='circle' type='light'>
                      {t('历史订阅')} ({historySubscriptions.length})
                    </Tag>
                  )}
                </div>
                {showSubscriptionOrderControls && (
                  <Text
                    type='tertiary'
                    size='small'
                    className='mt-1 block leading-5'
                  >
                    {t('拖动调整扣费顺序')}
                  </Text>
                )}
              </div>
              <div className='flex flex-wrap items-center justify-start gap-2 md:justify-end'>
                <Select
                  value={displayBillingPreference}
                  onChange={onChangeBillingPreference}
                  size='small'
                  optionList={[
                    {
                      value: 'subscription_first',
                      label: disableSubscriptionPreference
                        ? `${t('优先订阅')} (${t('无生效')})`
                        : t('优先订阅'),
                      disabled: disableSubscriptionPreference,
                    },
                    { value: 'wallet_first', label: t('优先钱包') },
                    {
                      value: 'subscription_only',
                      label: disableSubscriptionPreference
                        ? `${t('仅用订阅')} (${t('无生效')})`
                        : t('仅用订阅'),
                      disabled: disableSubscriptionPreference,
                    },
                    { value: 'wallet_only', label: t('仅用钱包') },
                  ]}
                />
                <Tooltip content={t('刷新')}>
                  <Button
                    aria-label={t('刷新')}
                    size='small'
                    theme='light'
                    type='tertiary'
                    icon={
                      <RefreshCw
                        size={12}
                        className={refreshing ? 'animate-spin' : ''}
                      />
                    }
                    onClick={handleRefresh}
                    loading={refreshing}
                  />
                </Tooltip>
              </div>
            </div>
            {disableSubscriptionPreference && isSubscriptionPreference && (
              <Text type='tertiary' size='small'>
                {t('已保存偏好为')}
                {subscriptionPreferenceLabel}
                {t('，当前无生效订阅，将自动使用钱包')}
              </Text>
            )}

            {hasAnySubscription ? (
              <>
                <Divider margin={8} />
                {activeVisibleSubscriptions.length > 0 && (
                  <SubscriptionDeductionOrderList
                    t={t}
                    subscriptions={activeVisibleSubscriptions}
                    showOrderControls={showSubscriptionOrderControls}
                    planTitleMap={planTitleMap}
                    savingOrder={savingOrder}
                    onReorder={reorderSubscriptions}
                  />
                )}
                {historySubscriptions.length > 0 && (
                  <Collapse className='mt-3'>
                    <Collapse.Panel
                      itemKey='history'
                      header={`${t('历史订阅')} (${historySubscriptions.length})`}
                    >
                      <SubscriptionHistoryList
                        t={t}
                        subscriptions={historySubscriptions}
                        planTitleMap={planTitleMap}
                      />
                    </Collapse.Panel>
                  </Collapse>
                )}
              </>
            ) : (
              <div className='text-xs text-gray-500'>
                {t('购买套餐后即可享受模型权益')}
              </div>
            )}
          </div>

          {/* 可购买套餐 - 标准定价卡片 */}
          {plans.length > 0 ? (
            <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 gap-5 w-full px-1'>
              {plans.map((p, index) => {
                const plan = p?.plan;
                const totalAmount = Number(plan?.total_amount || 0);
                const { symbol, rate } = getCurrencyConfig();
                const price = Number(plan?.price_amount || 0);
                const convertedPrice = price * rate;
                const displayPrice = convertedPrice.toFixed(
                  Number.isInteger(convertedPrice) ? 0 : 2,
                );
                const isPopular = index === 0 && plans.length > 1;
                const limit = Number(plan?.max_purchase_per_user || 0);
                const limitLabel = limit > 0 ? `${t('限购')} ${limit}` : null;
                const totalLabel =
                  totalAmount > 0
                    ? `${t('总额度')}: ${renderQuota(totalAmount)}`
                    : `${t('总额度')}: ${t('不限')}`;
                const upgradeLabel = plan?.upgrade_group
                  ? `${t('升级分组')}: ${plan.upgrade_group}`
                  : null;
                const resetLabel =
                  formatSubscriptionResetPeriod(plan, t) === t('不重置')
                    ? null
                    : `${t('额度重置')}: ${formatSubscriptionResetPeriod(plan, t)}`;
                const planBenefits = [
                  {
                    label: `${t('有效期')}: ${formatSubscriptionDuration(plan, t)}`,
                  },
                  resetLabel ? { label: resetLabel } : null,
                  totalAmount > 0
                    ? {
                        label: totalLabel,
                        tooltip: `${t('原生额度')}：${totalAmount}`,
                      }
                    : { label: totalLabel },
                  limitLabel ? { label: limitLabel } : null,
                  upgradeLabel ? { label: upgradeLabel } : null,
                ].filter(Boolean);

                return (
                  <Card
                    key={plan?.id}
                    className={`!rounded-xl transition-all hover:shadow-lg w-full h-full ${
                      isPopular ? 'ring-2 ring-purple-500' : ''
                    }`}
                    bodyStyle={{ padding: 0 }}
                  >
                    <div className='p-4 h-full flex flex-col'>
                      {/* 推荐标签 */}
                      {isPopular && (
                        <div className='mb-2'>
                          <Tag color='purple' shape='circle' size='small'>
                            <Sparkles size={10} className='mr-1' />
                            {t('推荐')}
                          </Tag>
                        </div>
                      )}
                      {/* 套餐名称 */}
                      <div className='mb-3'>
                        <Typography.Title
                          heading={5}
                          ellipsis={{ rows: 1, showTooltip: true }}
                          style={{ margin: 0 }}
                        >
                          {plan?.title || t('订阅套餐')}
                        </Typography.Title>
                        {plan?.subtitle && (
                          <Text
                            type='tertiary'
                            size='small'
                            ellipsis={{ rows: 1, showTooltip: true }}
                            style={{ display: 'block' }}
                          >
                            {plan.subtitle}
                          </Text>
                        )}
                      </div>

                      {/* 价格区域 */}
                      <div className='py-2'>
                        <div className='flex items-baseline justify-start'>
                          <span className='text-xl font-bold text-purple-600'>
                            {symbol}
                          </span>
                          <span className='text-3xl font-bold text-purple-600'>
                            {displayPrice}
                          </span>
                        </div>
                      </div>

                      {/* 套餐权益描述 */}
                      <div className='flex flex-col items-start gap-1 pb-2'>
                        {planBenefits.map((item) => {
                          const content = (
                            <div className='flex items-center gap-2 text-xs text-gray-500'>
                              <Badge dot type='tertiary' />
                              <span>{item.label}</span>
                            </div>
                          );
                          if (!item.tooltip) {
                            return (
                              <div
                                key={item.label}
                                className='w-full flex justify-start'
                              >
                                {content}
                              </div>
                            );
                          }
                          return (
                            <Tooltip key={item.label} content={item.tooltip}>
                              <div className='w-full flex justify-start'>
                                {content}
                              </div>
                            </Tooltip>
                          );
                        })}
                      </div>

                      <div className='mt-auto'>
                        <Divider margin={12} />

                        {/* 购买按钮 */}
                        {(() => {
                          const count = getPlanPurchaseCount(p?.plan?.id);
                          const reached = limit > 0 && count >= limit;
                          const tip = reached
                            ? t('已达到购买上限') + ` (${count}/${limit})`
                            : '';
                          const buttonEl = (
                            <Button
                              theme='outline'
                              type='primary'
                              block
                              disabled={reached}
                              onClick={() => {
                                if (!reached) openBuy(p);
                              }}
                            >
                              {reached ? t('已达上限') : t('立即订阅')}
                            </Button>
                          );
                          return reached ? (
                            <Tooltip content={tip} position='top'>
                              {buttonEl}
                            </Tooltip>
                          ) : (
                            buttonEl
                          );
                        })()}
                      </div>
                    </div>
                  </Card>
                );
              })}
            </div>
          ) : (
            <div className='text-center text-gray-400 text-sm py-4'>
              {t('暂无可购买套餐')}
            </div>
          )}
        </Space>
      )}
    </>
  );

  return (
    <>
      {withCard ? (
        <Card className='!rounded-2xl shadow-sm border-0'>{cardContent}</Card>
      ) : (
        <div className='space-y-3'>{cardContent}</div>
      )}

      {/* 购买确认弹窗 */}
      <SubscriptionPurchaseModal
        t={t}
        visible={open}
        onCancel={closeBuy}
        selectedPlan={selectedPlan}
        paying={paying}
        selectedEpayMethod={selectedEpayMethod}
        setSelectedEpayMethod={setSelectedEpayMethod}
        epayMethods={allPayMethods}
        enableOnlineTopUp={enableOnlineTopUp}
        enableStripeTopUp={enableStripeTopUp}
        enableCreemTopUp={enableCreemTopUp}
        purchaseLimitInfo={
          selectedPlan?.plan?.id
            ? {
                limit: Number(selectedPlan?.plan?.max_purchase_per_user || 0),
                count: getPlanPurchaseCount(selectedPlan?.plan?.id),
              }
            : null
        }
        onPayStripe={payStripe}
        onPayCreem={payCreem}
        onPayEpay={payEpay}
      />
    </>
  );
};

export default SubscriptionPlansCard;
