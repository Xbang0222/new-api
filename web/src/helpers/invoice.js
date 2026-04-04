/**
 * Invoice module API helpers (custom feature — no upstream counterpart)
 *
 * All invoice-related API calls go through here.
 * Components should never call API.get/post directly for invoice endpoints.
 */

import { API } from './api';

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
