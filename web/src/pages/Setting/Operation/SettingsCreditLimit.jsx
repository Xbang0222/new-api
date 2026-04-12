/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, Select } from '@douyinfe/semi-ui'; // custom: invite rebate (PR #3495) — added Select
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  showWarning,
  getCurrencyConfig,
} from '../../../helpers';

// custom: invite rebate (PR #3495) — keys this component manages
const FIELD_KEYS = [
  'QuotaForNewUser',
  'PreConsumedQuota',
  'QuotaForInviter',
  'QuotaForInvitee',
  'InviterRewardType',
  'InviterRewardValue',
  'MinAffTransferQuota', // custom: invite rebate anti-abuse
  'quota_setting.enable_free_model_pre_consume',
];

const BOOLEAN_KEYS = new Set(['quota_setting.enable_free_model_pre_consume']);

// custom: invite rebate anti-abuse — fields that need quota ↔ currency conversion on load/save
const QUOTA_TO_DOLLAR_KEYS = new Set(['MinAffTransferQuota']);

function getQuotaPerUnit() {
  const v = parseFloat(localStorage.getItem('quota_per_unit'));
  return v > 0 ? v : 500000;
}

/** Convert raw quota → display currency amount */
function quotaToDisplay(rawQuota) {
  const { rate } = getCurrencyConfig();
  return (rawQuota / getQuotaPerUnit()) * rate;
}

/** Convert display currency amount → raw quota */
function displayToQuota(displayAmount) {
  const { rate } = getCurrencyConfig();
  return Math.round((displayAmount / rate) * getQuotaPerUnit());
}

export default function SettingsCreditLimit(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [rewardType, setRewardType] = useState(''); // custom: invite rebate (PR #3495) — for dynamic UI
  const currencySymbol = getCurrencyConfig().symbol; // custom: invite rebate anti-abuse
  const refForm = useRef();
  const savedValues = useRef({}); // original values from API for diff comparison

  function onSubmit() {
    const formValues = refForm.current.getValues();

    // Build current values map (normalize types to strings for comparison)
    const current = {};
    for (const key of FIELD_KEYS) {
      const val = formValues[key];
      if (BOOLEAN_KEYS.has(key)) {
        current[key] = Boolean(val);
      } else {
        current[key] = String(val ?? '');
      }
    }

    // Compare with saved values
    const updateArray = [];
    for (const key of FIELD_KEYS) {
      const oldVal = savedValues.current[key];
      const newVal = current[key];
      if (String(oldVal ?? '') !== String(newVal ?? '')) {
        updateArray.push({
          key,
          value: BOOLEAN_KEYS.has(key) ? String(newVal) : newVal,
        });
      }
    }

    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));

    // custom: invite rebate (PR #3495) — validate rebate value range
    const rewardValueEntry = updateArray.find(
      (item) => item.key === 'InviterRewardValue',
    );
    if (rewardValueEntry) {
      const rewardValue = parseInt(rewardValueEntry.value);
      const currentType = current.InviterRewardType;
      if (isNaN(rewardValue)) {
        showError(t('充值返利值必须是有效的数字'));
        return;
      }
      if (
        currentType === 'percentage' &&
        (rewardValue < 0 || rewardValue > 100)
      ) {
        showError(t('当充值返利类型为百分比时，返利值应在0-100之间'));
        return;
      }
      if (currentType === 'fixed' && rewardValue < 0) {
        showError(t('当充值返利类型为固定时，返利值应大于等于0'));
        return;
      }
    }

    const requestQueue = updateArray.map((item) => {
      let value = item.value;
      // custom: invite rebate anti-abuse — convert currency input to raw quota for storage
      if (QUOTA_TO_DOLLAR_KEYS.has(item.key)) {
        const numVal = parseFloat(value) || 0;
        value = String(displayToQuota(numVal));
      }
      return API.put('/api/option/', { key: item.key, value });
    });

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (res.some((r) => !r || !r.data || !r.data.success)) {
          if (requestQueue.length === 1) return;
          return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (const key in props.options) {
      if (FIELD_KEYS.includes(key)) {
        let val = props.options[key];
        // custom: invite rebate anti-abuse — convert raw quota to display currency
        if (QUOTA_TO_DOLLAR_KEYS.has(key)) {
          const numVal = parseFloat(val) || 0;
          val = numVal > 0 ? String(quotaToDisplay(numVal)) : '0';
        }
        currentInputs[key] = val;
      }
    }
    savedValues.current = structuredClone(currentInputs);
    setRewardType(currentInputs.InviterRewardType || ''); // custom: invite rebate (PR #3495)
    if (refForm.current) {
      refForm.current.setValues(currentInputs);
    }
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('额度设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('新用户初始额度')}
                  field={'QuotaForNewUser'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  placeholder={''}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('请求预扣费额度')}
                  field={'PreConsumedQuota'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={t('请求结束后多退少补')}
                  placeholder={''}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('邀请新用户奖励额度')}
                  field={'QuotaForInviter'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：2000')}
                />
              </Col>
            </Row>
            <Row>
              <Col xs={24} sm={12} md={8} lg={8} xl={6}>
                <Form.InputNumber
                  label={t('新用户使用邀请码奖励额度')}
                  field={'QuotaForInvitee'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={''}
                  placeholder={t('例如：1000')}
                />
              </Col>
            </Row>
            <Row>
              <Col>
                <Form.Switch
                  label={t('对免费模型启用预消耗')}
                  field={'quota_setting.enable_free_model_pre_consume'}
                  extraText={t(
                    '开启后，对免费模型（倍率为0，或者价格为0）的模型也会预消耗额度',
                  )}
                />
              </Col>
            </Row>
          </Form.Section>

          {/* custom: invite rebate (PR #3495) */}
          <Form.Section text={t('邀请充值返利')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Select
                  label={t('充值返利类型')}
                  field={'InviterRewardType'}
                  extraText={t(
                    '设置被邀请人充值时，邀请人获得返利的类型。留空表示关闭返利功能',
                  )}
                  placeholder={t('关闭（不启用返利）')}
                  onChange={(value) => setRewardType(value || '')}
                  showClear
                >
                  <Select.Option value='fixed'>{t('固定额度')}</Select.Option>
                  <Select.Option value='percentage'>
                    {t('按充值比例')}
                  </Select.Option>
                </Form.Select>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={
                    t('充值返利值') +
                    (rewardType === 'percentage' ? ' (%)' : '')
                  }
                  field={'InviterRewardValue'}
                  step={1}
                  min={0}
                  max={rewardType === 'percentage' ? 100 : undefined}
                  suffix={rewardType === 'percentage' ? '%' : 'Token'}
                  extraText={
                    rewardType === 'percentage'
                      ? t(
                          '被邀请人每次充值时，邀请人获得充值额度的百分比作为返利',
                        )
                      : t('被邀请人每次充值时，邀请人获得的固定返利额度')
                  }
                  placeholder={
                    rewardType === 'percentage'
                      ? t('例如：10（表示10%）')
                      : t('例如：2000')
                  }
                  disabled={!rewardType}
                />
              </Col>
              {/* custom: invite rebate anti-abuse — minimum transfer threshold */}
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('最低划转门槛')}
                  field={'MinAffTransferQuota'}
                  step={0.5}
                  min={0}
                  suffix={currencySymbol}
                  extraText={t(
                    '返利余额达到此金额才允许划转，设为0则使用默认值（$1）。建议设置较高值防止小号刷返利',
                  )}
                  placeholder={t('例如：10')}
                />
              </Col>
            </Row>
          </Form.Section>

          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存额度设置')}
            </Button>
          </Row>
        </Form>
      </Spin>
    </>
  );
}
