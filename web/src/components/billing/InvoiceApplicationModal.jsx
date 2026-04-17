import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Modal,
  Form,
  Table,
  Typography,
  Select,
  Tag,
  Toast,
  Spin,
} from '@douyinfe/semi-ui';
import { API, showError, timestamp2string } from '../../helpers';
import { InvoiceAPI } from '../../helpers/invoice';

const InvoiceApplicationModal = ({
  visible,
  defaultContent,
  minAmount,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const [submitLoading, setSubmitLoading] = useState(false);
  const [topUpLoading, setTopUpLoading] = useState(false);
  const [headers, setHeaders] = useState([]);
  const [selectedHeaderId, setSelectedHeaderId] = useState(null);
  const [userEmail, setUserEmail] = useState('');
  const formRef = React.useRef();

  // 可开票的充值记录
  const [availableTopUps, setAvailableTopUps] = useState([]);
  // 跨页选中缓存：id -> TopUp（含 money），作为选中状态的 single source of truth
  const [selectedTopUpMap, setSelectedTopUpMap] = useState(() => new Map());
  const [topUpPage, setTopUpPage] = useState(1);
  const [topUpTotal, setTopUpTotal] = useState(0);

  const fetchAvailableTopUps = useCallback(async () => {
    setTopUpLoading(true);
    try {
      const res = await InvoiceAPI.getAvailableTopUps({
        p: topUpPage,
        page_size: 10,
      });
      if (res.data.success) {
        setAvailableTopUps(res.data.data.items || []);
        setTopUpTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setTopUpLoading(false);
    }
  }, [topUpPage]);

  const fetchHeaders = useCallback(async () => {
    try {
      const res = await InvoiceAPI.getHeaders();
      if (res.data.success) {
        const list = res.data.data || [];
        setHeaders(list);
        const defaultHeader = list.find((h) => h.is_default);
        if (defaultHeader && formRef.current) {
          setSelectedHeaderId(defaultHeader.id);
          formRef.current.formApi.setValues({
            company_name: defaultHeader.company_name,
            tax_number: defaultHeader.tax_number,
          });
        }
      }
    } catch {
      // ignore
    }
  }, []);

  // 获取用户邮箱自动填充
  const fetchUserEmail = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self');
      if (res.data.success && res.data.data.email) {
        setUserEmail(res.data.data.email);
        if (formRef.current) {
          formRef.current.formApi.setValue('email', res.data.data.email);
        }
      }
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    if (visible) {
      fetchAvailableTopUps();
      fetchHeaders();
      fetchUserEmail();
    }
  }, [visible, fetchAvailableTopUps, fetchHeaders, fetchUserEmail]);

  const handleHeaderSelect = (headerId) => {
    setSelectedHeaderId(headerId);
    const header = headers.find((h) => h.id === headerId);
    if (header && formRef.current) {
      formRef.current.formApi.setValues({
        company_name: header.company_name,
        tax_number: header.tax_number,
      });
    }
  };

  // 从缓存派生：驱动 Semi Table 的行高亮 + 提交时的 topup_ids
  const selectedRowKeys = useMemo(
    () => Array.from(selectedTopUpMap.keys()),
    [selectedTopUpMap],
  );

  // 跨页累计金额
  const selectedAmount = useMemo(() => {
    let sum = 0;
    for (const t of selectedTopUpMap.values()) {
      const v = Number(t.money);
      if (Number.isFinite(v)) sum += v;
    }
    return sum;
  }, [selectedTopUpMap]);

  // 只对"当前页的选中差异"应用到缓存，不影响其他页已选项
  const handleTopUpSelectionChange = useCallback(
    (newSelectedRowKeys) => {
      const currentPageIds = new Set(availableTopUps.map((t) => t.id));
      const nextPageSelected = new Set(newSelectedRowKeys);

      setSelectedTopUpMap((prev) => {
        const next = new Map(prev);
        for (const id of currentPageIds) {
          if (!nextPageSelected.has(id)) {
            next.delete(id);
          }
        }
        for (const topup of availableTopUps) {
          if (nextPageSelected.has(topup.id)) {
            next.set(topup.id, topup);
          }
        }
        return next;
      });
    },
    [availableTopUps],
  );

  const handleSubmit = async () => {
    if (selectedRowKeys.length === 0) {
      showError(t('请至少选择一条充值记录'));
      return;
    }
    if (selectedAmount < minAmount) {
      showError(`${t('最低开票金额为')} ¥${minAmount}`);
      return;
    }

    try {
      const values = await formRef.current.formApi.validate();
      setSubmitLoading(true);

      const res = await InvoiceAPI.submit({
        topup_ids: selectedRowKeys,
        email: values.email,
        company_name: values.company_name,
        tax_number: values.tax_number,
        content: defaultContent,
        remark: values.remark || '',
        header_id: selectedHeaderId || undefined,
      });

      if (res.data.success) {
        Toast.success(t('发票申请已提交'));
        onSuccess();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      if (err?.message) {
        showError(err.message);
      }
    } finally {
      setSubmitLoading(false);
    }
  };

  const topupColumns = [
    {
      title: t('交易号'),
      dataIndex: 'trade_no',
      render: (text) => (
        <Typography.Text
          ellipsis={{ showTooltip: true }}
          style={{ maxWidth: 200 }}
        >
          {text}
        </Typography.Text>
      ),
    },
    {
      title: t('支付方式'),
      dataIndex: 'payment_method',
      width: 90,
      render: (text) => <Tag size='small'>{text || '-'}</Tag>,
    },
    {
      title: t('实付金额'),
      dataIndex: 'money',
      width: 100,
      render: (val) => `¥ ${Number(val).toFixed(2)}`,
    },
    {
      title: t('充值时间'),
      dataIndex: 'create_time',
      width: 150,
      render: (val) => timestamp2string(val),
    },
  ];

  return (
    <Modal
      title={t('申请开票')}
      visible={visible}
      onCancel={onClose}
      onOk={handleSubmit}
      okText={t('提交申请')}
      cancelText={t('取消')}
      confirmLoading={submitLoading}
      okButtonProps={{
        disabled: selectedRowKeys.length === 0 || selectedAmount < minAmount,
      }}
      width={720}
      maskClosable={false}
    >
      {/* 步骤 1：选择账单 */}
      <Typography.Title heading={6} style={{ marginBottom: 8 }}>
        1. {t('选择充值账单')}
      </Typography.Title>

      <Spin spinning={topUpLoading}>
        <Table
          columns={topupColumns}
          dataSource={availableTopUps}
          rowKey='id'
          rowSelection={{
            selectedRowKeys,
            onChange: handleTopUpSelectionChange,
          }}
          pagination={
            topUpTotal > 10
              ? {
                  currentPage: topUpPage,
                  pageSize: 10,
                  total: topUpTotal,
                  onPageChange: setTopUpPage,
                  size: 'small',
                }
              : false
          }
          size='small'
          style={{ marginTop: 8, marginBottom: 4 }}
          empty={t('暂无可开票的充值记录')}
        />
      </Spin>

      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          padding: '8px 12px',
          background: 'var(--semi-color-fill-0)',
          borderRadius: 6,
          marginBottom: 16,
        }}
      >
        <span>
          {t('已选择')}: {selectedRowKeys.length} {t('笔')}
        </span>
        <span>
          {t('开票金额')}: <strong>¥ {selectedAmount.toFixed(2)}</strong>
          {selectedRowKeys.length > 0 && selectedAmount < minAmount && (
            <Typography.Text type='danger' style={{ marginLeft: 8 }}>
              （{t('最低开票金额为')} ¥{minAmount}）
            </Typography.Text>
          )}
        </span>
      </div>

      {/* 步骤 2：发票抬头 */}
      <Typography.Title heading={6} style={{ marginBottom: 8 }}>
        2. {t('填写发票抬头')}
      </Typography.Title>

      {headers.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <Select
            placeholder={t('选择已保存的抬头')}
            value={selectedHeaderId}
            onChange={handleHeaderSelect}
            style={{ width: '100%' }}
            optionList={headers.map((h) => ({
              value: h.id,
              label: `${h.company_name} (${h.tax_number})`,
            }))}
            showClear
            onClear={() => {
              setSelectedHeaderId(null);
              formRef.current?.formApi.setValues({
                company_name: '',
                tax_number: '',
              });
            }}
          />
        </div>
      )}

      <Form
        ref={formRef}
        labelPosition='top'
        initValues={{
          email: userEmail,
        }}
      >
        <div style={{ display: 'flex', gap: 12 }}>
          <div style={{ flex: 1 }}>
            <Form.Input
              field='company_name'
              label={t('单位名称')}
              placeholder={t('请输入单位全称')}
              rules={[{ required: true, message: t('请输入单位名称') }]}
            />
          </div>
          <div style={{ flex: 1 }}>
            <Form.Input
              field='tax_number'
              label={t('纳税人识别号')}
              placeholder={t('请输入18位统一社会信用代码')}
              rules={[
                { required: true, message: t('请输入纳税人识别号') },
                {
                  pattern: /^[0-9A-Z]{15,20}$/,
                  message: t('格式不正确'),
                },
              ]}
            />
          </div>
        </div>
        <div style={{ display: 'flex', gap: 12 }}>
          <div style={{ flex: 1 }}>
            <Form.Input
              field='email'
              label={t('接收邮箱')}
              placeholder={t('请输入接收发票的邮箱')}
              rules={[
                { required: true, message: t('请输入邮箱') },
                {
                  type: 'string',
                  pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
                  message: t('邮箱格式不正确'),
                },
              ]}
            />
          </div>
          <div style={{ flex: 1 }}>
            <Form.Input
              field='remark'
              label={t('开票备注')}
              placeholder={t('请简要说明用途')}
              maxLength={100}
            />
          </div>
        </div>

        {/* 发票内容 — 只读展示 */}
        <div
          style={{
            padding: '8px 12px',
            background: 'var(--semi-color-fill-0)',
            borderRadius: 6,
            marginTop: 4,
          }}
        >
          <Typography.Text type='tertiary' size='small'>
            {t('发票内容')}
          </Typography.Text>
          <Typography.Text style={{ display: 'block', marginTop: 2 }}>
            {defaultContent}
          </Typography.Text>
        </div>
      </Form>

      {selectedRowKeys.length > 0 && selectedAmount < minAmount && (
        <Typography.Text
          type='danger'
          size='small'
          style={{ display: 'block', marginTop: 8 }}
        >
          {t('最低开票金额为')} {minAmount} {t('元，当前已选金额不足')}
        </Typography.Text>
      )}
    </Modal>
  );
};

export default InvoiceApplicationModal;
