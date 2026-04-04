import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Table,
  Tag,
  Button,
  Space,
  Typography,
  Popconfirm,
  Toast,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { API, showError, timestamp2string } from '../../helpers';
import { InvoiceAPI } from '../../helpers/invoice';
import CardPro from '../../components/common/ui/CardPro';
import InvoiceApplicationModal from '../../components/billing/InvoiceApplicationModal';

const STATUS_MAP = {
  pending: { color: 'orange', text: '待支付' },
  success: { color: 'green', text: '已完成' },
  failed: { color: 'red', text: '失败' },
  expired: { color: 'grey', text: '已过期' },
};

const Billing = () => {
  const { t } = useTranslation();
  const [topups, setTopups] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [showInvoiceModal, setShowInvoiceModal] = useState(false);
  const [invoiceSetting, setInvoiceSetting] = useState(null);
  const [invoicedIds, setInvoicedIds] = useState(new Set());
  const [activeTab, setActiveTab] = useState('all');

  // 可开票记录
  const [availableTopUps, setAvailableTopUps] = useState([]);
  const [availablePage, setAvailablePage] = useState(1);
  const [availableTotal, setAvailableTotal] = useState(0);
  const [availableLoading, setAvailableLoading] = useState(false);

  const fetchTopUps = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/user/topup/self', {
        params: { p: page, page_size: pageSize },
      });
      if (res.data.success) {
        setTopups(res.data.data.items || []);
        setTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize]);

  const fetchSetting = useCallback(async () => {
    try {
      const res = await InvoiceAPI.getSetting();
      if (res.data.success) {
        setInvoiceSetting(res.data.data);
      }
    } catch {
      // ignore
    }
  }, []);

  const fetchInvoicedIds = useCallback(async () => {
    try {
      const res = await InvoiceAPI.getInvoicedTopUpIds();
      if (res.data.success) {
        setInvoicedIds(new Set(res.data.data || []));
      }
    } catch {
      // ignore
    }
  }, []);

  const fetchAvailableTopUps = useCallback(async () => {
    setAvailableLoading(true);
    try {
      const res = await InvoiceAPI.getAvailableTopUps({
        p: availablePage,
        page_size: pageSize,
      });
      if (res.data.success) {
        setAvailableTopUps(res.data.data.items || []);
        setAvailableTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setAvailableLoading(false);
    }
  }, [availablePage, pageSize]);

  useEffect(() => {
    fetchTopUps();
  }, [fetchTopUps]);

  useEffect(() => {
    fetchSetting();
    fetchInvoicedIds();
  }, [fetchSetting, fetchInvoicedIds]);

  useEffect(() => {
    if (activeTab === 'available') {
      fetchAvailableTopUps();
    }
  }, [activeTab, fetchAvailableTopUps]);

  const handleDeletePending = async (id) => {
    try {
      const res = await InvoiceAPI.deletePendingTopUp(id);
      if (res.data.success) {
        Toast.success(t('已删除'));
        fetchTopUps();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  // 按 Tab 过滤数据
  const getFilteredTopUps = () => {
    if (activeTab === 'invoiced') {
      return topups.filter((t) => invoicedIds.has(t.id));
    }
    return topups;
  };

  const baseColumns = [
    {
      title: t('订单号'),
      dataIndex: 'trade_no',
      render: (text) => (
        <Typography.Text
          copyable
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 240 }}
        >
          {text}
        </Typography.Text>
      ),
    },
    {
      title: t('支付方式'),
      dataIndex: 'payment_method',
      width: 100,
      render: (text) => <Tag size='small'>{text || '-'}</Tag>,
    },
    {
      title: t('支付金额'),
      dataIndex: 'money',
      width: 110,
      render: (val) => `¥ ${Number(val).toFixed(2)}`,
    },
  ];

  const allColumns = [
    ...baseColumns,
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 140,
      render: (status, record) => {
        const info = STATUS_MAP[status] || { color: 'grey', text: status };
        return (
          <Space>
            <Tag shape='circle' color={info.color} size='small'>
              {t(info.text)}
            </Tag>
            {invoicedIds.has(record.id) && (
              <Tag shape='circle' color='blue' size='small'>
                {t('已开票')}
              </Tag>
            )}
          </Space>
        );
      },
    },
    {
      title: t('创建时间'),
      dataIndex: 'create_time',
      width: 170,
      render: (val) => timestamp2string(val),
    },
    {
      title: t('操作'),
      width: 80,
      render: (_, record) =>
        record.status === 'pending' ? (
          <Popconfirm
            title={t('确定删除此待支付订单？')}
            onConfirm={() => handleDeletePending(record.id)}
          >
            <Button size='small' type='danger' theme='light'>
              {t('删除')}
            </Button>
          </Popconfirm>
        ) : null,
    },
  ];

  const simpleColumns = [
    ...baseColumns,
    {
      title: t('创建时间'),
      dataIndex: 'create_time',
      width: 170,
      render: (val) => timestamp2string(val),
    },
  ];

  const isEnabled = invoiceSetting?.enabled;

  const handleTabChange = (key) => {
    setActiveTab(key);
    setPage(1);
    setAvailablePage(1);
  };

  return (
    <div className='mt-[60px] px-2'>
      <CardPro
        type='type3'
        descriptionArea={
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}
          >
            <Typography.Title heading={5}>{t('充值账单')}</Typography.Title>
            {isEnabled && (
              <Button
                theme='solid'
                type='primary'
                onClick={() => setShowInvoiceModal(true)}
              >
                {t('申请开票')}
              </Button>
            )}
          </div>
        }
        tabsArea={
          <Tabs type='line' activeKey={activeTab} onChange={handleTabChange}>
            <TabPane tab={t('全部账单')} itemKey='all' />
            {isEnabled && <TabPane tab={t('未开票')} itemKey='available' />}
            {isEnabled && <TabPane tab={t('已开票')} itemKey='invoiced' />}
          </Tabs>
        }
        t={t}
      >
        {activeTab === 'all' && (
          <Table
            columns={allColumns}
            dataSource={topups}
            loading={loading}
            rowKey='id'
            className='rounded-xl overflow-hidden'
            pagination={{
              currentPage: page,
              pageSize,
              total,
              onPageChange: setPage,
              showTotal: true,
            }}
            empty={t('暂无充值记录')}
          />
        )}
        {activeTab === 'available' && (
          <Table
            columns={simpleColumns}
            dataSource={availableTopUps}
            loading={availableLoading}
            rowKey='id'
            className='rounded-xl overflow-hidden'
            pagination={{
              currentPage: availablePage,
              pageSize,
              total: availableTotal,
              onPageChange: setAvailablePage,
              showTotal: true,
            }}
            empty={t('暂无可开票的充值记录')}
          />
        )}
        {activeTab === 'invoiced' && (
          <Table
            columns={simpleColumns}
            dataSource={getFilteredTopUps()}
            loading={loading}
            rowKey='id'
            className='rounded-xl overflow-hidden'
            pagination={false}
            empty={t('暂无已开票记录')}
          />
        )}
      </CardPro>

      {showInvoiceModal && (
        <InvoiceApplicationModal
          visible={showInvoiceModal}
          defaultContent={invoiceSetting?.default_content || ''}
          minAmount={invoiceSetting?.min_amount || 500}
          onClose={() => setShowInvoiceModal(false)}
          onSuccess={() => {
            setShowInvoiceModal(false);
            fetchTopUps();
            fetchInvoicedIds();
            if (activeTab === 'available') fetchAvailableTopUps();
          }}
        />
      )}
    </div>
  );
};

export default Billing;
