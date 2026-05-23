// custom: invite reward log
import React, { useEffect, useState } from 'react';
import { Modal, Tabs, TabPane, Button } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import InvitationSummaryTable from './InvitationSummaryTable';
import InvitationLogsTable from './InvitationLogsTable';

const InvitationDetailModal = ({ visible, onClose }) => {
  const { t } = useTranslation();
  const [compact, setCompact] = useState(true);
  // refreshTick bumps whenever the modal opens so child tables refetch.
  const [refreshTick, setRefreshTick] = useState(0);

  useEffect(() => {
    if (visible) setRefreshTick((n) => n + 1);
  }, [visible]);

  return (
    <Modal
      title={t('邀请明细')}
      visible={visible}
      onCancel={onClose}
      footer={null}
      fullScreen
      maskClosable
      keepDOM={false}
    >
      <div
        style={{
          marginBottom: 8,
          display: 'flex',
          justifyContent: 'flex-end',
        }}
      >
        <Button size='small' onClick={() => setCompact((v) => !v)}>
          {compact ? t('自适应模式') : t('紧凑模式')}
        </Button>
      </div>

      <Tabs type='line' defaultActiveKey='summary' lazyRender>
        <TabPane tab={t('按人汇总')} itemKey='summary'>
          <InvitationSummaryTable
            t={t}
            compact={compact}
            refreshTick={refreshTick}
          />
        </TabPane>
        <TabPane tab={t('返利流水')} itemKey='logs'>
          <InvitationLogsTable
            t={t}
            compact={compact}
            refreshTick={refreshTick}
          />
        </TabPane>
      </Tabs>
    </Modal>
  );
};

export default InvitationDetailModal;
