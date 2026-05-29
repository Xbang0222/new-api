// custom: invite reward log
// 跟左侧订阅卡 / 右侧奖励说明 Card 同款 `<Card className='!rounded-xl w-full'>` 结构，
// 直接嵌在 InvitationCard 内部底部，自然跟父容器宽度同步。
import React, { useEffect, useMemo, useState } from 'react';
import {
  Card,
  Empty,
  Pagination,
  Tabs,
  TabPane,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../common/ui/CardTable';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  applyCompactColumns,
  renderTimestampNoWrap,
} from '../../helpers/customTable';
import { renderQuota } from '../../helpers/render';
import {
  fetchInvitationSummary,
  fetchInvitationLogs,
} from '../../helpers/invitation';

const { Text } = Typography;
const COMPACT_MODE = true; // 紧凑模式，按容器宽度分配，避免在 InvitationCard 内横向溢出
const PAGE_SIZE_OPTS = [10, 20, 50];

const renderRewardRule = (row) => {
  if (row.reward_type === 'percentage') return `${row.reward_value}%`;
  if (row.reward_type === 'fixed') return renderQuota(row.reward_value);
  return '-';
};

const InvitationDetailPanel = ({ t }) => {
  const isMobile = useIsMobile();
  const compactMode = COMPACT_MODE;
  const [activeTab, setActiveTab] = useState('summary');

  const [summaryItems, setSummaryItems] = useState([]);
  const [summaryTotal, setSummaryTotal] = useState(0);
  const [summaryPage, setSummaryPage] = useState(1);
  const [summaryPageSize, setSummaryPageSize] = useState(10);
  const [summaryLoading, setSummaryLoading] = useState(false);

  const [logItems, setLogItems] = useState([]);
  const [logTotal, setLogTotal] = useState(0);
  const [logPage, setLogPage] = useState(1);
  const [logPageSize, setLogPageSize] = useState(10);
  const [logLoading, setLogLoading] = useState(false);

  const loadSummary = async (p, ps) => {
    setSummaryLoading(true);
    try {
      const res = await fetchInvitationSummary(p, ps);
      if (res?.success) {
        setSummaryItems(res.data.items || []);
        setSummaryTotal(res.data.total || 0);
      }
    } finally {
      setSummaryLoading(false);
    }
  };

  const loadLogs = async (p, ps) => {
    setLogLoading(true);
    try {
      const res = await fetchInvitationLogs(p, ps);
      if (res?.success) {
        setLogItems(res.data.items || []);
        setLogTotal(res.data.total || 0);
      }
    } finally {
      setLogLoading(false);
    }
  };

  useEffect(() => {
    loadSummary(1, summaryPageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (activeTab === 'logs' && logItems.length === 0 && !logLoading) {
      loadLogs(1, logPageSize);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab]);

  const summaryColumns = useMemo(
    () => [
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (v, row) =>
          v ? (
            <Text>{row.display_name || v}</Text>
          ) : (
            <Text type='tertiary'>
              {t('已注销用户')} #{row.invitee_id}
            </Text>
          ),
      },
      {
        title: t('累计充值'),
        dataIndex: 'total_recharge',
        render: (v) => <Text>{renderQuota(v || 0)}</Text>,
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
        fixed: 'right',
        render: (v) => (v ? renderTimestampNoWrap(v) : <Text>-</Text>),
      },
    ],
    [t],
  );

  const logsColumns = useMemo(
    () => [
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (v, row) =>
          v ? (
            <Text>{row.display_name || v}</Text>
          ) : (
            <Text type='tertiary'>
              {t('已注销用户')} #{row.invitee_id}
            </Text>
          ),
      },
      {
        title: t('充值额'),
        dataIndex: 'recharge_quota',
        render: (v) => <Text>{renderQuota(v || 0)}</Text>,
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
        render: (_v, row) => <Text>{renderRewardRule(row)}</Text>,
      },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        fixed: 'right',
        render: renderTimestampNoWrap,
      },
    ],
    [t],
  );

  const summaryTableColumns = useMemo(
    () => applyCompactColumns(summaryColumns, compactMode),
    [summaryColumns, compactMode],
  );
  const logsTableColumns = useMemo(
    () => applyCompactColumns(logsColumns, compactMode),
    [logsColumns, compactMode],
  );

  const emptyEl = (desc) => (
    <Empty
      image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
      darkModeImage={
        <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
      }
      description={desc}
      style={{ padding: 20 }}
    />
  );

  const renderPagination = (currentPage, pageSize, total, onPageChange, onPageSizeChange) => {
    if (!total || total <= 0) return null;
    const start = (currentPage - 1) * pageSize + 1;
    const end = Math.min(currentPage * pageSize, total);
    return (
      <div
        className={`mt-3 pt-3 flex w-full border-t ${isMobile ? 'justify-center' : 'justify-between items-center'}`}
        style={{ borderColor: 'var(--semi-color-border)' }}
      >
        {!isMobile && (
          <span
            className='text-sm select-none'
            style={{ color: 'var(--semi-color-text-2)' }}
          >
            {`${t('显示第')} ${start} ${t('条 - 第')} ${end} ${t('条，共')} ${total} ${t('条')}`}
          </span>
        )}
        <Pagination
          currentPage={currentPage}
          pageSize={pageSize}
          total={total}
          pageSizeOpts={PAGE_SIZE_OPTS}
          showSizeChanger
          onPageChange={onPageChange}
          onPageSizeChange={onPageSizeChange}
          size={isMobile ? 'small' : 'default'}
          showQuickJumper={isMobile}
          showTotal
        />
      </div>
    );
  };

  return (
    <Card
      className='!rounded-xl w-full'
      title={<Text type='tertiary'>{t('邀请明细')}</Text>}
    >
      <Tabs
        type='line'
        activeKey={activeTab}
        onChange={setActiveTab}
        tabBarStyle={{ marginBottom: 16 }}
      >
        <TabPane tab={t('邀请人员')} itemKey='summary'>
          <CardTable
            columns={summaryTableColumns}
            dataSource={summaryItems}
            loading={summaryLoading}
            rowKey='invitee_id'
            size='small'
            scroll={compactMode ? undefined : { x: 'max-content' }}
            className='rounded-xl overflow-hidden'
            empty={emptyEl(t('暂无邀请记录'))}
            hidePagination
          />
          {renderPagination(
            summaryPage,
            summaryPageSize,
            summaryTotal,
            (p) => {
              setSummaryPage(p);
              loadSummary(p, summaryPageSize);
            },
            (ps) => {
              setSummaryPageSize(ps);
              setSummaryPage(1);
              loadSummary(1, ps);
            },
          )}
        </TabPane>
        <TabPane tab={t('返利明细')} itemKey='logs'>
          <CardTable
            columns={logsTableColumns}
            dataSource={logItems}
            loading={logLoading}
            rowKey='id'
            size='small'
            scroll={compactMode ? undefined : { x: 'max-content' }}
            className='rounded-xl overflow-hidden'
            empty={emptyEl(t('暂无返利记录'))}
            hidePagination
          />
          {renderPagination(
            logPage,
            logPageSize,
            logTotal,
            (p) => {
              setLogPage(p);
              loadLogs(p, logPageSize);
            },
            (ps) => {
              setLogPageSize(ps);
              setLogPage(1);
              loadLogs(1, ps);
            },
          )}
        </TabPane>
      </Tabs>
    </Card>
  );
};

export default InvitationDetailPanel;
