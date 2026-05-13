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

// custom: invoice ui — 充值/账单/发票表格双模式工具集
//
// 集中放 ruoli fork 自定义表格的共享工具,避免散落在多个文件、也避免修改上游文件
// (`hooks/common/useTableCompactMode.js`、`helpers/render.jsx`)。
//
// 使用场景:接入 usage-logs 同款"紧凑/自适应"双模式表格,见:
//   - pages/Billing/index.js (充值账单 3 个 Tab)
//   - pages/Invoice/index.js (我的发票)
//   - components/topup/modals/TopupHistoryModal.jsx (充值账单弹窗)
//   - components/invoice/InvoiceHeaderManager.jsx (抬头管理)
//   - components/billing/InvoiceApplicationModal.jsx (申请开票弹窗)

import React from 'react';
import { timestamp2string } from './utils';

/**
 * 紧凑模式下剥掉列上的 fixed 属性,让列按容器宽度分配。
 * 自适应模式下保留 fixed,配合 scroll={{ x: 'max-content' }} 实现锁列 + 横向滚动。
 *
 * 用法:
 *   const tableColumns = useMemo(
 *     () => applyCompactColumns(columns, compactMode),
 *     [columns, compactMode],
 *   );
 *
 * @param {Array} columns 原始列定义数组
 * @param {boolean} compactMode 当前是否紧凑模式
 * @returns {Array} 处理后的列定义
 */
export function applyCompactColumns(columns, compactMode) {
  return compactMode
    ? columns.map(({ fixed, ...rest }) => rest)
    : columns;
}

/**
 * 时间戳列的标准 render:强制 nowrap 防止 `YYYY-MM-DD HH:mm:ss` 在窄列里换行。
 * 配合 max-content 后,列宽自动按内容撑开。
 *
 * 用法:
 *   { title: '创建时间', dataIndex: 'create_time', render: renderTimestampNoWrap }
 *
 * @param {number} val unix timestamp (秒)
 * @returns {JSX.Element}
 */
export function renderTimestampNoWrap(val) {
  return (
    <span style={{ whiteSpace: 'nowrap' }}>{timestamp2string(val)}</span>
  );
}
