// custom: invite reward log
import { API } from './api';

export const fetchInvitationSummary = async (page = 1, pageSize = 20) => {
  const res = await API.get('/api/user/invites/summary', {
    params: { page, page_size: pageSize },
  });
  return res.data;
};

export const fetchInvitationLogs = async (page = 1, pageSize = 20) => {
  const res = await API.get('/api/user/invites/logs', {
    params: { page, page_size: pageSize },
  });
  return res.data;
};
