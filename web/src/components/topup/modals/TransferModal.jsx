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
import { Modal, Typography, Input, InputNumber } from '@douyinfe/semi-ui';
import { CreditCard } from 'lucide-react';
import { quotaToDisplayAmount } from '../../../helpers/quota';
import { getCurrencyConfig } from '../../../helpers/render';

const TransferModal = ({
  t,
  openTransfer,
  transfer,
  handleTransferCancel,
  userState,
  renderQuota,
  getQuotaPerUnit,
  transferAmount,
  setTransferAmount,
  minTransferQuota = 0, // custom: invite rebate anti-abuse — admin-configured minimum (raw quota)
}) => {
  const { symbol } = getCurrencyConfig();
  const minRaw = minTransferQuota > 0 ? minTransferQuota : getQuotaPerUnit();
  const minDisplay = quotaToDisplayAmount(minRaw);
  const maxDisplay = quotaToDisplayAmount(userState?.user?.aff_quota || 0);

  return (
    <Modal
      title={
        <div className='flex items-center'>
          <CreditCard className='mr-2' size={18} />
          {t('划转邀请额度')}
        </div>
      }
      visible={openTransfer}
      onOk={transfer}
      onCancel={handleTransferCancel}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('可用邀请额度')}
          </Typography.Text>
          <Input
            value={renderQuota(userState?.user?.aff_quota)}
            disabled
            className='!rounded-lg'
          />
        </div>
        <div>
          <Typography.Text strong className='block mb-2'>
            {t('划转额度')} · {t('最低') + ' ' + symbol + minDisplay.toFixed(2)}
          </Typography.Text>
          <InputNumber
            min={minDisplay}
            max={maxDisplay}
            step={0.5}
            precision={2}
            value={transferAmount}
            onChange={(value) => setTransferAmount(value)}
            suffix={symbol}
            className='w-full !rounded-lg'
          />
        </div>
      </div>
    </Modal>
  );
};

export default TransferModal;
