/*
Copyright (C) 2023-2026 QuantumNous

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
import type { TFunction } from 'i18next'

import type { NameRule, ModelStatus, SyncSource } from './types'

// ============================================================================
// Pagination
// ============================================================================

export const DEFAULT_PAGE_SIZE = 20

// ============================================================================
// Name Rule Options
// ============================================================================

export function getNameRuleOptions(t: TFunction) {
  return [
    { label: t('Exact Match'), value: 0 as NameRule },
    { label: t('Prefix Match'), value: 1 as NameRule },
    { label: t('Contains Match'), value: 2 as NameRule },
    { label: t('Suffix Match'), value: 3 as NameRule },
  ] as const
}

export function getNameRuleConfig(
  t: TFunction
): Record<NameRule, { label: string; color: string; description: string }> {
  return {
    0: {
      label: t('Exact'),
      color: 'green',
      description: t('Match model name exactly'),
    },
    1: {
      label: t('Prefix'),
      color: 'blue',
      description: t('Match models starting with this name'),
    },
    2: {
      label: t('Contains'),
      color: 'orange',
      description: t('Match models containing this name'),
    },
    3: {
      label: t('Suffix'),
      color: 'purple',
      description: t('Match models ending with this name'),
    },
  }
}

// ============================================================================
// Model Status
// ============================================================================

export function getModelStatusOptions(t: TFunction) {
  return [
    { label: t('All Status'), value: 'all' },
    { label: t('Shown'), value: 'enabled' },
    { label: t('Not shown'), value: 'disabled' },
  ] as const
}

export function getModelStatusConfig(
  t: TFunction
): Record<ModelStatus, { label: string; variant: 'success' | 'neutral' }> {
  return {
    1: { label: t('Shown'), variant: 'success' },
    0: { label: t('Not shown'), variant: 'neutral' },
  }
}

// ============================================================================
// Sync Status Options
// ============================================================================

export function getSyncStatusOptions(t: TFunction) {
  return [
    { label: t('All Sync Status'), value: 'all' },
    { label: t('Official Sync'), value: 'yes' },
    { label: t('No Sync'), value: 'no' },
  ] as const
}

// ============================================================================
// Deployment Status
// ============================================================================

export function getDeploymentStatusOptions(t: TFunction) {
  return [
    { label: t('All Status'), value: 'all' },
    { label: t('Running'), value: 'running' },
    { label: t('Completed'), value: 'completed' },
    { label: t('Failed'), value: 'failed' },
    { label: t('Deployment requested'), value: 'deployment requested' },
    { label: t('Termination requested'), value: 'termination requested' },
    { label: t('Destroyed'), value: 'destroyed' },
  ] as const
}

export function getDeploymentStatusConfig(t: TFunction): Record<
  string,
  {
    label: string
    variant: 'success' | 'neutral' | 'warning' | 'danger'
  }
> {
  return {
    running: { label: t('Running'), variant: 'success' },
    completed: { label: t('Completed'), variant: 'success' },
    failed: { label: t('Failed'), variant: 'danger' },
    error: { label: t('Failed'), variant: 'danger' },
    destroyed: { label: t('Destroyed'), variant: 'danger' },
    'deployment requested': {
      label: t('Deployment requested'),
      variant: 'warning',
    },
    'termination requested': {
      label: t('Termination requested'),
      variant: 'warning',
    },
  }
}

// ============================================================================
// Quota Type
// ============================================================================

export function getQuotaTypeConfig(
  t: TFunction
): Record<number, { label: string; color: string }> {
  return {
    0: { label: t('Usage-based'), color: 'violet' },
    1: { label: t('Per-call'), color: 'teal' },
  }
}

// ============================================================================
// Endpoint Templates
// ============================================================================

export type EndpointOption = {
  type: string
  path: string
  method: string
  labelKey?: string
  labelKeys?: readonly string[]
  label: string
}

export const ENDPOINT_OPTIONS = [
  {
    type: 'openai',
    path: '/v1/chat/completions',
    method: 'POST',
    labelKey: 'OpenAI Chat Completions',
    label: 'OpenAI Chat Completions',
  },
  {
    type: 'openai-completions',
    path: '/v1/completions',
    method: 'POST',
    labelKey: 'OpenAI Completions',
    label: 'OpenAI Completions',
  },
  {
    type: 'openai-response',
    path: '/v1/responses',
    method: 'POST',
    labelKey: 'OpenAI Responses',
    label: 'OpenAI Responses',
  },
  {
    type: 'openai-response-compact',
    path: '/v1/responses/compact',
    method: 'POST',
    labelKey: 'OpenAI Responses Compact',
    label: 'OpenAI Responses Compact',
  },
  {
    type: 'anthropic',
    path: '/v1/messages',
    method: 'POST',
    labelKey: 'Anthropic',
    label: 'Anthropic',
  },
  {
    type: 'gemini',
    path: '/v1beta/models/{model}:generateContent',
    method: 'POST',
    labelKey: 'Gemini Generate Content',
    label: 'Gemini Generate Content',
  },
  {
    type: 'jina-rerank',
    path: '/v1/rerank',
    method: 'POST',
    labelKey: 'Rerank',
    label: 'Rerank',
  },
  {
    type: 'image-generation',
    path: '/v1/images/generations',
    method: 'POST',
    labelKey: 'OpenAI Image Generations',
    label: 'OpenAI Image Generations',
  },
  {
    type: 'image-edits',
    path: '/v1/images/edits',
    method: 'POST',
    labelKey: 'OpenAI Image Edits',
    label: 'OpenAI Image Edits',
  },
  {
    type: 'embeddings',
    path: '/v1/embeddings',
    method: 'POST',
    labelKey: 'OpenAI Embeddings',
    label: 'OpenAI Embeddings',
  },
  {
    type: 'audio-speech',
    path: '/v1/audio/speech',
    method: 'POST',
    labelKey: 'OpenAI Audio Speech',
    label: 'OpenAI Audio Speech',
  },
  {
    type: 'audio-transcriptions',
    path: '/v1/audio/transcriptions',
    method: 'POST',
    labelKey: 'OpenAI Audio Transcriptions',
    label: 'OpenAI Audio Transcriptions',
  },
  {
    type: 'audio-translations',
    path: '/v1/audio/translations',
    method: 'POST',
    labelKey: 'OpenAI Audio Translations',
    label: 'OpenAI Audio Translations',
  },
  {
    type: 'moderations',
    path: '/v1/moderations',
    method: 'POST',
    label: 'OpenAI Moderations',
  },
  {
    type: 'realtime',
    path: '/v1/realtime',
    method: 'GET',
    labelKey: 'OpenAI Realtime',
    label: 'OpenAI Realtime',
  },
  {
    type: 'openai-video',
    path: '/v1/videos',
    method: 'POST',
    labelKeys: ['Video', 'Task'],
    label: 'Video Task',
  },
  {
    type: 'openai-video-retrieve',
    path: '/v1/videos/{task_id}',
    method: 'GET',
    labelKeys: ['Video', 'Task', 'Status'],
    label: 'Video Task Status',
  },
  {
    type: 'openai-video-content',
    path: '/v1/videos/{task_id}/content',
    method: 'GET',
    labelKeys: ['Video', 'Content'],
    label: 'Video Content',
  },
  {
    type: 'openai-video-remix',
    path: '/v1/videos/{video_id}/remix',
    method: 'POST',
    labelKey: 'Video Remix',
    label: 'Video Remix',
  },
] as const

export const ENDPOINT_TEMPLATES: Record<
  string,
  { path: string; method: string }
> = ENDPOINT_OPTIONS.reduce(
  (acc, endpoint) => {
    acc[endpoint.type] = {
      path: endpoint.path,
      method: endpoint.method,
    }
    return acc
  },
  {} as Record<string, { path: string; method: string }>
)

// ============================================================================
// Sync Locale Options
// ============================================================================

export function getSyncLocaleOptions(t: TFunction) {
  return [
    { label: t('Chinese'), value: 'zh' },
    { label: t('English'), value: 'en' },
    { label: t('Japanese'), value: 'ja' },
  ] as const
}

export function getSyncSourceOptions(t: TFunction) {
  return [
    {
      label: t('Official Repository'),
      value: 'official' as SyncSource,
      description: t('Sync from the public upstream metadata repository.'),
      disabled: false,
    },
  ] as const
}
