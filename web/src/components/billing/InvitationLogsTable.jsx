// custom: invite reward log
import React, { useEffect, useState } from 'react';
import { Table, Spin, Empty, Typography, Banner } from '@douyinfe/semi-ui';
import { fetchInvitationLogs } from '../../helpers/invitation';
import { timestamp2string } from '../../helpers/utils';
import { renderQuota } from '../../helpers/render';

const { Text } = Typography;

const renderRewardRule = (row) => {
  if (row.reward_type === 'percentage') return `${row.reward_value}%`;
  if (row.reward_type === 'fixed') return renderQuota(row.reward_value);
  return '-';
};

const InvitationLogsTable = ({ t, compact, refreshTick }) => {
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const load = async (p = page, ps = pageSize) => {
    setLoading(true);
    try {
      const res = await fetchInvitationLogs(p, ps);
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
      title: t('时间'),
      dataIndex: 'created_at',
      render: (v) => timestamp2string(v),
    },
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
      title: t('充值额'),
      dataIndex: 'recharge_quota',
      render: (v) => renderQuota(v || 0),
    },
    {
      title: t('返利额'),
      dataIndex: 'reward_quota',
      render: (v) => (
        <Text strong type='success'>
          {renderQuota(v || 0)}
        </Text>
      ),
    },
    {
      title: t('返利规则'),
      dataIndex: 'reward_type',
      render: (_v, row) => renderRewardRule(row),
    },
  ];

  return (
    <Spin spinning={loading}>
      <Banner
        type='info'
        description={t('邀请明细_流水说明')}
        closeIcon={null}
        style={{ marginBottom: 12 }}
      />
      <Table
        size={compact ? 'small' : 'middle'}
        columns={columns}
        dataSource={items}
        rowKey='id'
        empty={<Empty description={t('暂无返利记录')} />}
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

export default InvitationLogsTable;
