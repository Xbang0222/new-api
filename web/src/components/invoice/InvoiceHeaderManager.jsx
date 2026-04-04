import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Table,
  Button,
  Modal,
  Form,
  Tag,
  Toast,
  Popconfirm,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { InvoiceAPI } from '../../helpers/invoice';
import { showError } from '../../helpers';

const InvoiceHeaderManager = () => {
  const { t } = useTranslation();
  const [headers, setHeaders] = useState([]);
  const [loading, setLoading] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [editingHeader, setEditingHeader] = useState(null);
  const formRef = React.useRef();

  const fetchHeaders = useCallback(async () => {
    setLoading(true);
    try {
      const res = await InvoiceAPI.getHeaders();
      if (res.data.success) {
        setHeaders(res.data.data || []);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchHeaders();
  }, [fetchHeaders]);

  const handleSave = async () => {
    try {
      const values = await formRef.current.formApi.validate();
      if (editingHeader) {
        const res = await InvoiceAPI.updateHeader(editingHeader.id, values);
        if (res.data.success) {
          Toast.success(t('抬头已更新'));
        } else {
          showError(res.data.message);
          return;
        }
      } else {
        const res = await InvoiceAPI.createHeader(values);
        if (res.data.success) {
          Toast.success(t('抬头已保存'));
        } else {
          showError(res.data.message);
          return;
        }
      }
      setShowModal(false);
      setEditingHeader(null);
      fetchHeaders();
    } catch {
      // validation error
    }
  };

  const handleDelete = async (id) => {
    try {
      const res = await InvoiceAPI.deleteHeader(id);
      if (res.data.success) {
        Toast.success(t('抬头已删除'));
        fetchHeaders();
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
      title: t('纳税人识别号'),
      dataIndex: 'tax_number',
      render: (text) => <Typography.Text copyable>{text}</Typography.Text>,
    },
    {
      title: t('默认'),
      dataIndex: 'is_default',
      width: 80,
      render: (val) => (val ? <Tag color='blue'>{t('默认')}</Tag> : null),
    },
    {
      title: t('操作'),
      width: 150,
      render: (_, record) => (
        <Space>
          <Button
            size='small'
            theme='light'
            onClick={() => {
              setEditingHeader(record);
              setShowModal(true);
            }}
          >
            {t('编辑')}
          </Button>
          <Popconfirm
            title={t('确定删除此抬头？')}
            onConfirm={() => handleDelete(record.id)}
          >
            <Button size='small' type='danger' theme='light'>
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ marginTop: 12 }}>
      <div style={{ marginBottom: 12 }}>
        <Button
          theme='solid'
          onClick={() => {
            setEditingHeader(null);
            setShowModal(true);
          }}
        >
          {t('新建抬头')}
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={headers}
        loading={loading}
        rowKey='id'
        className='rounded-xl overflow-hidden'
        pagination={false}
        empty={t('暂无保存的抬头模板')}
      />

      <Modal
        title={editingHeader ? t('编辑抬头') : t('新建抬头')}
        visible={showModal}
        onCancel={() => {
          setShowModal(false);
          setEditingHeader(null);
        }}
        onOk={handleSave}
        okText={t('保存')}
        cancelText={t('取消')}
        maskClosable={false}
      >
        <Form
          ref={formRef}
          labelPosition='top'
          initValues={
            editingHeader
              ? {
                  company_name: editingHeader.company_name,
                  tax_number: editingHeader.tax_number,
                  is_default: editingHeader.is_default,
                }
              : { is_default: false }
          }
        >
          <Form.Input
            field='company_name'
            label={t('单位名称')}
            placeholder={t('请输入单位全称')}
            rules={[{ required: true, message: t('请输入单位名称') }]}
          />
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
          <Form.Switch field='is_default' label={t('设为默认')} />
        </Form>
      </Modal>
    </div>
  );
};

export default InvoiceHeaderManager;
