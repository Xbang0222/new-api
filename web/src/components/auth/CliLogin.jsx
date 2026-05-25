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

import React, { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Card, Spin, Typography } from '@douyinfe/semi-ui';
import { API, copy, showSuccess } from '../../helpers';

const { Text, Title, Paragraph } = Typography;

const SESSION_KEY = 'newapi.cli_login.params';
const POLL_INTERVAL_MS = 2000;
const POLL_MAX_ATTEMPTS = 150; // 150 * 2s = 5 minutes

const STATUS = {
  STARTING: 'starting',
  AWAITING_LOGIN: 'awaitingLogin',
  EXCHANGING: 'exchanging',
  RETURNING: 'returning',
  DONE: 'done',
  ERROR: 'error',
};

// Prefer fresh query params; fall back to sessionStorage so OAuth
// round-trips (which strip the query) still find port/state.
function readParams(searchParams) {
  const port = searchParams.get('port');
  const state = searchParams.get('state');
  if (port && state) {
    try {
      sessionStorage.setItem(SESSION_KEY, JSON.stringify({ port, state }));
    } catch {
      /* sessionStorage may be unavailable in private browsing */
    }
    return { port, state };
  }
  try {
    return JSON.parse(sessionStorage.getItem(SESSION_KEY) || 'null');
  } catch {
    return null;
  }
}

const CliLogin = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [status, setStatus] = useState(STATUS.STARTING);
  const [errMsg, setErrMsg] = useState('');
  // Guard against React 18 Strict Mode double-invocation.
  const hasExecuted = useRef(false);

  useEffect(() => {
    if (hasExecuted.current) return;
    hasExecuted.current = true;

    let cancelled = false;

    const sleep = (ms) =>
      new Promise((resolve) => {
        setTimeout(resolve, ms);
      });

    const isLoggedIn = async () => {
      try {
        const res = await API.get('/api/user/self', {
          skipErrorHandler: true,
        });
        return Boolean(res?.data?.success);
      } catch {
        return false;
      }
    };

    (async () => {
      const params = readParams(searchParams);
      if (!params || !params.port || !params.state) {
        if (cancelled) return;
        setStatus(STATUS.ERROR);
        setErrMsg(t('缺少 port/state 参数。请通过 CLI 启动浏览器登录。'));
        return;
      }

      let loggedIn = await isLoggedIn();
      if (cancelled) return;

      if (!loggedIn) {
        setStatus(STATUS.AWAITING_LOGIN);
        for (let i = 0; i < POLL_MAX_ATTEMPTS && !loggedIn; i++) {
          await sleep(POLL_INTERVAL_MS);
          if (cancelled) return;
          loggedIn = await isLoggedIn();
          if (cancelled) return;
        }
      }
      if (!loggedIn) {
        setStatus(STATUS.ERROR);
        setErrMsg(t('5 分钟内未检测到登录状态，请登录后刷新本页面重试。'));
        return;
      }

      setStatus(STATUS.EXCHANGING);
      let exchangeRes;
      try {
        exchangeRes = await API.post(
          '/api/user/cli-login/exchange',
          { state: params.state },
          { skipErrorHandler: true },
        );
      } catch (e) {
        if (cancelled) return;
        setStatus(STATUS.ERROR);
        const msg = e?.response?.data?.message || e?.message || t('未知错误');
        setErrMsg(t('换取 CLI 令牌失败：') + msg);
        return;
      }
      if (cancelled) return;
      if (!exchangeRes?.data?.success) {
        setStatus(STATUS.ERROR);
        setErrMsg(exchangeRes?.data?.message || t('换取 CLI 令牌失败'));
        return;
      }

      setStatus(STATUS.RETURNING);
      try {
        const cbResp = await fetch(`http://127.0.0.1:${params.port}/callback`, {
          method: 'POST',
          mode: 'cors',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(exchangeRes.data.data),
        });
        if (!cbResp.ok) {
          throw new Error(`loopback HTTP ${cbResp.status}`);
        }
      } catch (e) {
        if (cancelled) return;
        setStatus(STATUS.ERROR);
        setErrMsg(
          t('无法回传到 CLI 本地端口：') + (e?.message || t('未知错误')),
        );
        return;
      }

      try {
        sessionStorage.removeItem(SESSION_KEY);
      } catch {
        /* ignore */
      }
      if (cancelled) return;
      setStatus(STATUS.DONE);
    })();

    return () => {
      cancelled = true;
    };
  }, [searchParams, t]);

  const copyError = async () => {
    const ok = await copy(errMsg);
    if (ok) {
      showSuccess(t('已复制到剪贴板'));
    }
  };

  return (
    <div className='flex items-center justify-center min-h-screen p-6'>
      <Card style={{ maxWidth: 560, width: '100%' }}>
        <Title heading={3} style={{ marginTop: 0 }}>
          {t('CLI 登录')}
        </Title>

        {status === STATUS.STARTING && (
          <div className='flex items-center gap-2'>
            <Spin />
            <Text>{t('正在检查登录状态...')}</Text>
          </div>
        )}

        {status === STATUS.AWAITING_LOGIN && (
          <>
            <Paragraph>
              {t('请在另一个标签页登录本站，登录完成后本页会自动继续。')}
            </Paragraph>
            <Paragraph>
              <a href='/login' target='_blank' rel='noreferrer'>
                {t('打开登录页')}
              </a>
            </Paragraph>
            <div className='flex items-center gap-2'>
              <Spin />
              <Text type='tertiary'>{t('正在等待登录...')}</Text>
            </div>
          </>
        )}

        {status === STATUS.EXCHANGING && (
          <div className='flex items-center gap-2'>
            <Spin />
            <Text>{t('正在换取 CLI 令牌...')}</Text>
          </div>
        )}

        {status === STATUS.RETURNING && (
          <div className='flex items-center gap-2'>
            <Spin />
            <Text>{t('正在把令牌回传到 CLI...')}</Text>
          </div>
        )}

        {status === STATUS.DONE && (
          <>
            <Paragraph>
              <Text strong>{t('登录成功。')}</Text>{' '}
              {t('您可以关闭此页面并返回终端继续操作。')}
            </Paragraph>
          </>
        )}

        {status === STATUS.ERROR && (
          <>
            <Paragraph>
              <Text type='danger' strong>
                {t('出错了：')}
              </Text>{' '}
              <Text>{errMsg}</Text>
            </Paragraph>
            <Button onClick={copyError} disabled={!errMsg}>
              {t('复制错误信息')}
            </Button>
          </>
        )}
      </Card>
    </div>
  );
};

export default CliLogin;
