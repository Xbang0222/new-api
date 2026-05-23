// custom: invite reward log
import React, { useEffect, useState } from 'react';
import { Table, Spin, Empty, Typography } from '@douyinfe/semi-ui';
import { fetchInvitationSummary } from '../../helpers/invitation';
import { timestamp2string } from '../../helpers/utils';
import { renderQuota } from '../../helpers/render';

const { Text } = Typography;

const InvitationSummaryTable = ({ t, compact, refreshTick }) => {
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const load = async (p = page, ps = pageSize) => {
    setLoading(true);
    try {
      const res = await fetchInvitationSummary(p, ps);
      if (res?.success) {
        setItems(res.data.items || []);
        setTotal(res.data.total || 0);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load(1, pageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshTick]);

  const columns = [
    {
      title: t('用户'),
      dataIndex: 'username',
      render: (v, row) =>
        v ? (
          <span>{row.display_name || v}</span>
        ) : (
          <Text type='tertiary'>
            {t('已注销用户')} #{row.invitee_id}
          </Text>
        ),
    },
    {
      title: t('注册时间'),
      dataIndex: 'invited_at',
      render: (v) => (v ? timestamp2string(v) : '-'),
    },
    {
      title: t('累计充值'),
      dataIndex: 'total_recharge',
      render: (v) => renderQuota(v || 0),
    },
    {
      title: t('累计返利'),
      dataIndex: 'total_reward',
      render: (v) => (
        <Text strong type={v > 0 ? 'success' : 'tertiary'}>
          {renderQuota(v || 0)}
        </Text>
      ),
    },
    {
      title: t('最后返利时间'),
      dataIndex: 'last_reward_at',
      render: (v) => (v ? timestamp2string(v) : '-'),
    },
  ];

  return (
    <Spin spinning={loading}>
      <Table
        size={compact ? 'small' : 'middle'}
        columns={columns}
        dataSource={items}
        rowKey='invitee_id'
        empty={<Empty description={t('邀请明细_空状态')} />}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOpts: [10, 20, 50],
          onPageChange: (p) => {
            setPage(p);
            load(p, pageSize);
          },
          onPageSizeChange: (ps) => {
            setPageSize(ps);
            setPage(1);
            load(1, ps);
          },
        }}
      />
    </Spin>
  );
};

export default InvitationSummaryTable;
