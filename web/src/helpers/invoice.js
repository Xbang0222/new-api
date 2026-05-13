/**
 * Invoice module API helpers (custom feature — no upstream counterpart)
 *
 * All invoice-related API calls go through here.
 * Components should never call API.get/post directly for invoice endpoints.
 */

import { API } from './api';
import { renderQuota } from './render';
import { getQuotaPerUnit } from './quota';

/**
 * Format an invoice amount (stored as RMB yuan on the backend) for display,
 * following the project-wide currency display preference (USD / CNY / CUSTOM /
 * TOKENS) set in localStorage.quota_display_type.
 *
 * Reuses the project's `renderQuota` helper so currency symbol, exchange rate,
 * custom-currency config and rounding all stay in sync with the rest of the UI
 * — admin changes to `usd_exchange_rate` and `quota_per_unit` flow through
 * automatically, no hardcoded rate of our own.
 *
 * Flow: RMB → USD (divide by usd_exchange_rate) → quota → renderQuota(quota)
 */
export const formatInvoiceAmount = (rmbAmount) => {
  const rmb = Number(rmbAmount || 0);
  if (!Number.isFinite(rmb)) return renderQuota(0, 2);

  // Fallback 1 mirrors renderQuota's own CNY branch when status is missing.
  let usdRate = 1;
  try {
    const statusStr = localStorage.getItem('status');
    if (statusStr) {
      const s = JSON.parse(statusStr);
      const rate = Number(s?.usd_exchange_rate);
      if (Number.isFinite(rate) && rate > 0) usdRate = rate;
    }
  } catch (e) {}

  const quotaPerUnit = getQuotaPerUnit();
  const quota = (rmb / usdRate) * quotaPerUnit;
  return renderQuota(quota, 2);
};

export const InvoiceAPI = {
  // ---- 配置 ----
  getSetting: () => API.get('/api/invoice/setting'),

  // ---- 可开票充值记录 ----
  getAvailableTopUps: (params) =>
    API.get('/api/invoice/available-topups', { params }),
  getInvoicedTopUpIds: () => API.get('/api/invoice/invoiced-topup-ids'),

  // ---- 发票申请 ----
  submit: (data) => API.post('/api/invoice/', data),
  getMyInvoices: (params) => API.get('/api/invoice/self', { params }),
  cancel: (id) => API.put(`/api/invoice/${id}/cancel`),
  getItems: (id) => API.get(`/api/invoice/${id}/items`),

  // ---- 发票抬头模板 ----
  getHeaders: () => API.get('/api/invoice/headers'),
  createHeader: (data) => API.post('/api/invoice/headers', data),
  updateHeader: (id, data) => API.put(`/api/invoice/headers/${id}`, data),
  deleteHeader: (id) => API.delete(`/api/invoice/headers/${id}`),

  // ---- 管理员 ----
  getAllInvoices: (params) => API.get('/api/invoice/admin/', { params }),
  review: (id, data) => API.put(`/api/invoice/admin/${id}/review`, data),
  markSent: (id) => API.put(`/api/invoice/admin/${id}/send`),

  // ---- 充值记录 ----
  deletePendingTopUp: (id) => API.delete(`/api/invoice/topup/${id}`),
};
