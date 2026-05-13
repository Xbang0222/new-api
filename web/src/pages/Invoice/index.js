import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Tag,
  Button,
  Typography,
  Popconfirm,
  Toast,
  Space,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { InvoiceAPI, formatInvoiceAmount } from '../../helpers/invoice';
import { showError } from '../../helpers';
import { createCardProPagination } from '../../helpers/utils';
import {
  applyCompactColumns,
  renderTimestampNoWrap,
} from '../../helpers/customTable';
import {
  INVOICE_STATUS,
  INVOICE_STATUS_LABEL,
  INVOICE_STATUS_COLOR,
} from '../../constants/invoice.constants';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import CompactModeToggle from '../../components/common/ui/CompactModeToggle';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useTableCompactMode } from '../../hooks/common/useTableCompactMode';
import InvoiceHeaderManager from '../../components/invoice/InvoiceHeaderManager';

const Invoice = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [activeTab, setActiveTab] = useState('invoices');

  // custom: invoice ui — usage-logs 同款双模式
  const [compactMode, setCompactMode] = useTableCompactMode('invoices');

  const fetchInvoices = useCallback(async () => {
    setLoading(true);
    try {
      const res = await InvoiceAPI.getMyInvoices({
        p: page,
        page_size: pageSize,
      });
      if (res.data.success) {
        setInvoices(res.data.data.items || []);
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

  useEffect(() => {
    fetchInvoices();
  }, [fetchInvoices]);

  const handleCancel = async (invoiceId) => {
    try {
      const res = await InvoiceAPI.cancel(invoiceId);
      if (res.data.success) {
        Toast.success(t('发票申请已撤销'));
        fetchInvoices();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const columns = [
    {
      title: t('单位名称'),
      dataIndex: 'company_name',
      render: (text) => <Typography.Text>{text}</Typography.Text>,
    },
    {
      title: t('税号'),
      dataIndex: 'tax_number',
      render: (text) => <Typography.Text copyable>{text}</Typography.Text>,
    },
    {
      title: t('金额'),
      dataIndex: 'amount',
      width: 120,
      render: (val) => formatInvoiceAmount(val),
    },
    {
      // custom: invoice fee — show fee snapshot + refund status
      title: t('开票服务费'),
      dataIndex: 'fee_amount',
      width: 130,
      render: (val, record) => {
        const fee = Number(val) || 0;
        if (fee <= 0) return '-';
        return (
          <Typography.Text size='small'>
            {formatInvoiceAmount(fee)}
            {record.fee_refunded && (
              <Tag size='small' color='blue' style={{ marginLeft: 4 }}>
                {t('开票服务费已退还')}
              </Tag>
            )}
          </Typography.Text>
        );
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (status) => (
        <Tag shape='circle' color={INVOICE_STATUS_COLOR[status]} size='small'>
          {t(INVOICE_STATUS_LABEL[status] || '未知')}
        </Tag>
      ),
    },
    {
      title: t('申请时间'),
      dataIndex: 'create_time',
      render: renderTimestampNoWrap,
    },
    {
      title: t('操作'),
      width: 120,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          {record.status === INVOICE_STATUS.PENDING && (
            <Popconfirm
              title={t('确定撤销此发票申请？')}
              onConfirm={() => handleCancel(record.id)}
            >
              <Button size='small' type='danger' theme='light'>
                {t('撤销')}
              </Button>
            </Popconfirm>
          )}
          {record.status === INVOICE_STATUS.REJECTED &&
            record.reject_reason && (
              <Typography.Text type='danger' style={{ fontSize: 12 }}>
                {record.reject_reason}
              </Typography.Text>
            )}
        </Space>
      ),
    },
  ];

  // custom: invoice ui — columns 是普通字面量(每次 render 重建),inline 使用 helper
  // 而非 useMemo:依赖不稳定时 useMemo 等价于 inline,反而误导
  const tableColumns = applyCompactColumns(columns, compactMode);

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
            <Typography.Title heading={5}>{t('发票管理')}</Typography.Title>
            {activeTab === 'invoices' && (
              <CompactModeToggle
                compactMode={compactMode}
                setCompactMode={setCompactMode}
                t={t}
              />
            )}
          </div>
        }
        tabsArea={
          <Tabs type='line' activeKey={activeTab} onChange={setActiveTab}>
            <TabPane tab={t('发票记录')} itemKey='invoices' />
            <TabPane tab={t('发票抬头')} itemKey='headers' />
          </Tabs>
        }
        paginationArea={
          activeTab === 'invoices'
            ? createCardProPagination({
                currentPage: page,
                pageSize,
                total,
                onPageChange: setPage,
                isMobile,
                showSizeChanger: false,
                t,
              })
            : null
        }
        t={t}
      >
        {activeTab === 'invoices' && (
          // custom: invoice — CardTable + CardPro.paginationArea 模式,跟 usage-logs
          // 一致;Table 自身 rounded-xl 处理表格主体,分页栏交给 CardPro 槽,跟随 !rounded-2xl。
          <CardTable
            columns={tableColumns}
            dataSource={invoices}
            loading={loading}
            rowKey='id'
            scroll={compactMode ? undefined : { x: 'max-content' }}
            className='rounded-xl overflow-hidden'
            size='small'
            empty={t('暂无发票记录')}
            hidePagination
          />
        )}
        {activeTab === 'headers' && <InvoiceHeaderManager />}
      </CardPro>
    </div>
  );
};

export default Invoice;
