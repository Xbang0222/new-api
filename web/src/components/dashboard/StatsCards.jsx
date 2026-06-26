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

import React from 'react';
import { Card, Avatar, Skeleton, Tag } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const StatsCards = ({
  groupedStatsData,
  loading,
  getTrendSpec,
  CARD_PROPS,
  CHART_CONFIG,
}) => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const shouldShowTrend = (trendData) =>
    loading ||
    (Array.isArray(trendData) &&
      trendData.some((value) => Number(value) > 0));

  return (
    <>
      {groupedStatsData.map((group, idx) => (
        <Card
          key={idx}
          {...CARD_PROPS}
          className={`dashboard-card dashboard-summary-card dashboard-metric-card ${group.color}`}
          title={<div className='dashboard-card-title'>{group.title}</div>}
        >
          <div className='dashboard-metric-list'>
            {group.items.map((item, itemIdx) => (
              <button
                key={itemIdx}
                type='button'
                className='dashboard-metric-row'
                onClick={item.onClick}
              >
                <span className='dashboard-metric-left'>
                  <Avatar
                    className='dashboard-metric-avatar'
                    size='small'
                    color={item.avatarColor}
                  >
                    {item.icon}
                  </Avatar>
                  <span className='dashboard-metric-copy'>
                    <span className='dashboard-metric-label'>{item.title}</span>
                    <span className='dashboard-metric-value'>
                      <Skeleton
                        loading={loading}
                        active
                        placeholder={
                            <Skeleton.Paragraph
                              active
                              rows={1}
                              style={{
                                width: '65px',
                                height: '24px',
                                marginTop: '4px',
                              }}
                            />
                        }
                      >
                        {item.value}
                      </Skeleton>
                    </span>
                  </span>
                </span>
                {item.title === t('当前余额') ? (
                  <Tag
                    className='dashboard-chip'
                    color='white'
                    shape='circle'
                    size='large'
                    onClick={(e) => {
                      e.stopPropagation();
                      navigate('/console/topup');
                    }}
                  >
                    {t('充值')}
                  </Tag>
                ) : (
                  shouldShowTrend(item.trendData) && (
                    <span className='dashboard-sparkline'>
                      <VChart
                        spec={getTrendSpec(item.trendData, item.trendColor)}
                        option={CHART_CONFIG}
                      />
                    </span>
                  )
                )}
              </button>
            ))}
          </div>
        </Card>
      ))}
    </>
  );
};

export default StatsCards;
