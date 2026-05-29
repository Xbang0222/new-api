// custom: invitation sidebar
// 把"邀请奖励"从充值页(components/topup/index.jsx)拆出来，成为独立左侧栏页面，
// 自带划转弹窗 + 邀请链接拉取逻辑（原先散在 TopUp 组件里），与额度充值解耦。
import React, { useContext, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
  copy,
  getQuotaPerUnit,
} from '../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../helpers/quota';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import InvitationCard from './InvitationCard';
import TransferModal from './modals/TransferModal';

const InvitationPanel = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);
  const affFetchedRef = useRef(false);

  const getUserQuota = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      let link = `${window.location.origin}/register?aff=${data}`;
      setAffLink(link);
    } else {
      showError(message);
    }
  };

  const transfer = async () => {
    const quotaAmount = displayAmountToQuota(transferAmount);
    if (quotaAmount < getQuotaPerUnit()) {
      showError(t('划转金额最低为') + ' ' + renderQuota(getQuotaPerUnit()));
      return;
    }
    const res = await API.post(`/api/user/aff_transfer`, {
      quota: quotaAmount,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      getUserQuota().then();
    } else {
      showError(message);
    }
  };

  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  const handleTransferCancel = () => {
    setOpenTransfer(false);
  };

  useEffect(() => {
    getUserQuota().then();
    const minRaw =
      statusState?.status?.min_aff_transfer_quota || getQuotaPerUnit();
    setTransferAmount(quotaToDisplayAmount(minRaw));
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink().then();
  }, []);

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2'>
      <TransferModal
        t={t}
        openTransfer={openTransfer}
        transfer={transfer}
        handleTransferCancel={handleTransferCancel}
        userState={userState}
        renderQuota={renderQuota}
        getQuotaPerUnit={getQuotaPerUnit}
        transferAmount={transferAmount}
        setTransferAmount={setTransferAmount}
        minTransferQuota={statusState?.status?.min_aff_transfer_quota || 0}
      />
      <InvitationCard
        t={t}
        userState={userState}
        renderQuota={renderQuota}
        setOpenTransfer={setOpenTransfer}
        affLink={affLink}
        handleAffLinkClick={handleAffLinkClick}
        statusState={statusState} // custom: invite rebate (PR #3495)
      />
    </div>
  );
};

export default InvitationPanel;
