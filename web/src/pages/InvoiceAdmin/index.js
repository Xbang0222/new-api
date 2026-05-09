import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Table,
  Tag,
  Button,
  Input,
  Select,
  Typography,
  Space,
  Modal,
  Form,
  Toast,
  Popconfirm,
  Tooltip,
} from '@douyinfe/semi-ui';
import { IconInfoCircle } from '@douyinfe/semi-icons';
import { InvoiceAPI } from '../../helpers/invoice';
import { showError, timestamp2string } from '../../helpers';
import CardPro from '../../components/common/ui/CardPro';
import {
  INVOICE_STATUS,
  INVOICE_STATUS_LABEL,
  INVOICE_STATUS_COLOR,
  INVOICE_STATUS_OPTIONS,
} from '../../constants/invoice.constants';

const InvoiceAdmin = () => {
  const { t } = useTranslation();
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [statusFilter, setStatusFilter] = useState(0);
  const [keyword, setKeyword] = useState('');

  // Review modal
  const [reviewVisible, setReviewVisible] = useState(false);
  const [reviewInvoice, setReviewInvoice] = useState(null);
  const [reviewLoading, setReviewLoading] = useState(false);
  const reviewFormRef = React.useRef();

  const fetchInvoices = useCallback(async () => {
    setLoading(true);
    try {
      const params = { p: page, page_size: pageSize };
      if (statusFilter > 0) params.status = statusFilter;
      if (keyword) params.keyword = keyword;

      const res = await InvoiceAPI.getAllInvoices(params);
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
  }, [page, pageSize, statusFilter, keyword]);

  useEffect(() => {
    fetchInvoices();
  }, [fetchInvoices]);

  const handleMarkSent = async (invoiceId) => {
    try {
      const res = await InvoiceAPI.markSent(invoiceId);
      if (res.data.success) {
        Toast.success(t('已标记为已发送'));
        fetchInvoices();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const openReview = (record) => {
    setReviewInvoice(record);
    setReviewVisible(true);
  };

  const handleReview = async (action) => {
    setReviewLoading(true);
    try {
      const data = { action };
      if (action === 'reject') {
        const values = await reviewFormRef.current?.formApi?.validate();
        data.reject_reason = values?.reject_reason || '';
      }

      const res = await InvoiceAPI.review(reviewInvoice.id, data);
      if (res.data.success) {
        Toast.success(action === 'approve' ? t('已通过') : t('已拒绝'));
        setReviewVisible(false);
        setReviewInvoice(null);
        fetchInvoices();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      if (err?.message) showError(err.message);
    } finally {
      setReviewLoading(false);
    }
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
    },
    {
      title: t('用户ID'),
      dataIndex: 'user_id',
      width: 80,
    },
    {
      title: t('单位名称'),
      dataIndex: 'company_name',
      width: 140,
      render: (text) => (
        <Typography.Text ellipsis={{ showTooltip: true }}>
          {text}
        </Typography.Text>
      ),
    },
    {
      title: t('税号'),
      dataIndex: 'tax_number',
      render: (text) => (
        <Typography.Text copyable ellipsis={{ showTooltip: true }}>
          {text}
        </Typography.Text>
      ),
    },
    {
      title: t('邮箱'),
      dataIndex: 'email',
      render: (text) => (
        <Typography.Text copyable ellipsis={{ showTooltip: true }}>
          {text || '-'}
        </Typography.Text>
      ),
    },
    {
      title: t('金额'),
      dataIndex: 'amount',
      width: 90,
      render: (val) => `¥ ${Number(val).toFixed(2)}`,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 80,
      render: (status) => (
        <Tag color={INVOICE_STATUS_COLOR[status]}>
          {t(INVOICE_STATUS_LABEL[status] || '未知')}
        </Tag>
      ),
    },
    {
      title: t('申请时间'),
      dataIndex: 'create_time',
      width: 110,
      render: (val) => {
        const full = timestamp2string(val);
        const date = full.split(' ')[0];
        return <Tooltip content={full}>{date}</Tooltip>;
      },
    },
    {
      title: t('操作'),
      width: 130,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          {record.status === INVOICE_STATUS.PENDING && (
            <Button
              size='small'
              theme='solid'
              onClick={() => openReview(record)}
            >
              {t('审核')}
            </Button>
          )}
          {record.status === INVOICE_STATUS.APPROVED && (
            <Popconfirm
              title={t('确认发票已发送给用户？')}
              onConfirm={() => handleMarkSent(record.id)}
            >
              <Button size='small' theme='solid' type='tertiary'>
                {t('标记已发送')}
              </Button>
            </Popconfirm>
          )}
          {record.status === INVOICE_STATUS.REJECTED &&
            record.reject_reason && (
              <Tooltip content={record.reject_reason}>
                <IconInfoCircle
                  style={{ color: 'var(--semi-color-danger)', cursor: 'help' }}
                />
              </Tooltip>
            )}
        </Space>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <CardPro
        type='type2'
        statsArea={
          <Typography.Title heading={5}>{t('发票审核')}</Typography.Title>
        }
        searchArea={
          <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
            <Select
              value={statusFilter}
              onChange={setStatusFilter}
              style={{ width: 160 }}
              optionList={INVOICE_STATUS_OPTIONS.map((o) => ({
                ...o,
                label: t(o.label),
              }))}
            />
            <Input
              placeholder={t('搜索单位名称或税号')}
              value={keyword}
              onChange={setKeyword}
              onEnterPress={fetchInvoices}
              showClear
              style={{ width: 240 }}
            />
          </div>
        }
        t={t}
      >
        <Table
          columns={columns}
          dataSource={invoices}
          loading={loading}
          rowKey='id'
          className='rounded-xl overflow-hidden'
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage: page,
            pageSize,
            total,
            onPageChange: setPage,
            showTotal: true,
          }}
          empty={t('暂无发票申请')}
        />
      </CardPro>

      {/* 审核弹窗 */}
      <Modal
        title={t('审核发票申请')}
        visible={reviewVisible}
        onCancel={() => {
          setReviewVisible(false);
          setReviewInvoice(null);
        }}
        footer={
          <Space>
            <Button
              onClick={() => {
                setReviewVisible(false);
                setReviewInvoice(null);
              }}
            >
              {t('取消')}
            </Button>
            <Button
              type='danger'
              theme='solid'
              loading={reviewLoading}
              onClick={() => handleReview('reject')}
            >
              {t('拒绝')}
            </Button>
            <Button
              type='primary'
              theme='solid'
              loading={reviewLoading}
              onClick={() => handleReview('approve')}
            >
              {t('通过')}
            </Button>
          </Space>
        }
        maskClosable={false}
        width={500}
      >
        {reviewInvoice && (
          <div>
            <div style={{ marginBottom: 12 }}>
              <strong>{t('单位名称')}：</strong>
              {reviewInvoice.company_name}
            </div>
            <div style={{ marginBottom: 12 }}>
              <strong>{t('税号')}：</strong>
              {reviewInvoice.tax_number}
            </div>
            <div style={{ marginBottom: 12 }}>
              <strong>{t('金额')}：</strong>¥{' '}
              {Number(reviewInvoice.amount).toFixed(2)}
            </div>
            {Number(reviewInvoice.fee_amount) > 0 && (
              // custom: invoice fee — display fee snapshot + refund status
              <div style={{ marginBottom: 12 }}>
                <strong>{t('服务费')}：</strong>¥{' '}
                {Number(reviewInvoice.fee_amount).toFixed(2)}
                <Typography.Text type='tertiary' size='small' style={{ marginLeft: 6 }}>
                  ({(Number(reviewInvoice.fee_rate) * 100).toFixed(2)}%
                  {reviewInvoice.fee_refunded
                    ? ` · ${t('已退还')}`
                    : ` · ${t('已扣除')}`}
                  )
                </Typography.Text>
              </div>
            )}
            <div style={{ marginBottom: 12 }}>
              <strong>{t('发票内容')}：</strong>
              {reviewInvoice.content}
            </div>
            {reviewInvoice.remark && (
              <div style={{ marginBottom: 12 }}>
                <strong>{t('备注')}：</strong>
                {reviewInvoice.remark}
              </div>
            )}

            <Form ref={reviewFormRef} labelPosition='top'>
              <Form.TextArea
                field='reject_reason'
                label={t('拒绝原因（拒绝时必填）')}
                placeholder={t('请输入拒绝原因')}
                maxCount={255}
              />
            </Form>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default InvoiceAdmin;
