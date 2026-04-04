/**
 * Invoice module constants (custom feature — no upstream counterpart)
 */

export const INVOICE_STATUS = {
  PENDING: 1,
  APPROVED: 2,
  REJECTED: 3,
  CANCELED: 4,
  SENT: 5,
};

export const INVOICE_STATUS_LABEL = {
  [INVOICE_STATUS.PENDING]: '待审核',
  [INVOICE_STATUS.APPROVED]: '已通过',
  [INVOICE_STATUS.REJECTED]: '已拒绝',
  [INVOICE_STATUS.CANCELED]: '已撤销',
  [INVOICE_STATUS.SENT]: '已发送',
};

export const INVOICE_STATUS_COLOR = {
  [INVOICE_STATUS.PENDING]: 'orange',
  [INVOICE_STATUS.APPROVED]: 'green',
  [INVOICE_STATUS.REJECTED]: 'red',
  [INVOICE_STATUS.CANCELED]: 'grey',
  [INVOICE_STATUS.SENT]: 'blue',
};

/**
 * Status options for admin filter dropdown
 */
export const INVOICE_STATUS_OPTIONS = [
  { value: 0, label: '全部状态' },
  { value: INVOICE_STATUS.PENDING, label: '待审核' },
  { value: INVOICE_STATUS.APPROVED, label: '已通过' },
  { value: INVOICE_STATUS.REJECTED, label: '已拒绝' },
  { value: INVOICE_STATUS.CANCELED, label: '已撤销' },
  { value: INVOICE_STATUS.SENT, label: '已发送' },
];
