import React, { useEffect, useState, useRef } from 'react';
import { Button, Form, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import { useTranslation } from 'react-i18next';

/**
 * 发票设置组件（custom — 无上游对应文件）
 * 管理 invoice_setting.* 配置项
 */
export default function InvoiceSettingComponent() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [inputs, setInputs] = useState({
    enabled: false,
    min_amount: 500,
    default_content: '*信息技术服务*技术服务费',
    max_headers: 5,
  });
  const formApiRef = useRef(null);

  // 加载当前配置
  useEffect(() => {
    const fetchOptions = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/option/');
        if (res.data.success) {
          const options = res.data.data;
          const newInputs = { ...inputs };
          for (const item of options) {
            switch (item.key) {
              case 'invoice_setting.enabled':
                newInputs.enabled = item.value === 'true';
                break;
              case 'invoice_setting.min_amount':
                newInputs.min_amount = parseFloat(item.value) || 500;
                break;
              case 'invoice_setting.default_content':
                newInputs.default_content =
                  item.value || '*信息技术服务*技术服务费';
                break;
              case 'invoice_setting.max_headers':
                newInputs.max_headers = parseInt(item.value) || 5;
                break;
            }
          }
          setInputs(newInputs);
          if (formApiRef.current) {
            formApiRef.current.setValues(newInputs);
          }
        }
      } catch (error) {
        showError(t('加载配置失败'));
      } finally {
        setLoading(false);
      }
    };
    fetchOptions();
  }, []);

  // 保存单个配置项
  const saveOption = async (key, value) => {
    const res = await API.put('/api/option/', {
      key: `invoice_setting.${key}`,
      value: String(value),
    });
    if (!res.data.success) {
      throw new Error(res.data.message);
    }
  };

  const handleSubmit = async () => {
    setSaving(true);
    try {
      await Promise.all([
        saveOption('enabled', inputs.enabled),
        saveOption('min_amount', inputs.min_amount),
        saveOption('default_content', inputs.default_content),
        saveOption('max_headers', inputs.max_headers),
      ]);
      showSuccess(t('保存成功'));
    } catch (error) {
      showError(error.message || t('保存失败'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={(values) => setInputs(values)}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('发票功能设置')}>
          <Typography.Text
            type='tertiary'
            style={{ marginBottom: 16, display: 'block' }}
          >
            {t(
              '启用后，用户可在"充值账单"页面申请开票，管理员在"发票审核"中审批',
            )}
          </Typography.Text>
          <Form.Switch field='enabled' label={t('启用发票功能')} />
          <Form.InputNumber
            field='min_amount'
            label={t('最低开票金额（元）')}
            min={0}
            step={100}
            style={{ width: 200 }}
          />
          <Form.Input
            field='default_content'
            label={t('默认发票内容')}
            placeholder='*信息技术服务*技术服务费'
          />
          <Form.InputNumber
            field='max_headers'
            label={t('每用户最多抬头模板数')}
            min={1}
            max={20}
            style={{ width: 200 }}
          />
        </Form.Section>
        <Button
          type='primary'
          theme='solid'
          onClick={handleSubmit}
          loading={saving}
          style={{ marginTop: 16 }}
        >
          {t('保存发票设置')}
        </Button>
      </Form>
    </Spin>
  );
}
