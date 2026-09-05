/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { api } from '@/lib/api'

import type {
  CreateInvoiceApplicationRequest,
  EligibleInvoiceOrder,
  InvoiceApiResponse,
  InvoiceApplication,
  InvoiceApplicationPage,
  InvoiceConfig,
  InvoicePaymentMethod,
  InvoicePaymentResponse,
  ReviewInvoiceApplicationRequest,
} from './types'

export async function getInvoiceConfig(): Promise<
  InvoiceApiResponse<InvoiceConfig>
> {
  const response = await api.get('/api/invoice/config')
  return response.data
}

export async function getEligibleInvoiceOrders(): Promise<
  InvoiceApiResponse<EligibleInvoiceOrder[]>
> {
  const response = await api.get('/api/invoice/eligible-orders')
  return response.data
}

export async function getInvoicePaymentMethods(): Promise<
  InvoiceApiResponse<InvoicePaymentMethod[]>
> {
  const response = await api.get('/api/invoice/payment-methods')
  return response.data
}

export async function getOwnInvoiceApplications(
  page: number,
  pageSize: number,
  filters?: { status?: string; keyword?: string }
): Promise<InvoiceApiResponse<InvoiceApplicationPage>> {
  const response = await api.get('/api/invoice/self', {
    params: {
      p: page,
      page_size: pageSize,
      status: filters?.status || undefined,
      keyword: filters?.keyword || undefined,
    },
  })
  return response.data
}

export async function getAdminInvoiceApplications(
  page: number,
  pageSize: number,
  filters?: { status?: string; keyword?: string }
): Promise<InvoiceApiResponse<InvoiceApplicationPage>> {
  const response = await api.get('/api/invoice/admin/applications', {
    params: {
      p: page,
      page_size: pageSize,
      status: filters?.status || undefined,
      keyword: filters?.keyword || undefined,
    },
  })
  return response.data
}

export async function createInvoiceApplication(
  request: CreateInvoiceApplicationRequest
): Promise<InvoiceApiResponse<InvoiceApplication>> {
  const response = await api.post('/api/invoice/applications', request)
  return response.data
}

export async function cancelInvoiceApplication(
  applicationId: number
): Promise<InvoiceApiResponse<null>> {
  const response = await api.post(
    `/api/invoice/applications/${applicationId}/cancel`
  )
  return response.data
}

export async function requestInvoiceSupplementPayment(
  applicationId: number,
  paymentMethod: string
): Promise<InvoicePaymentResponse> {
  const response = await api.post(
    `/api/invoice/applications/${applicationId}/pay`,
    { payment_method: paymentMethod },
    { skipBusinessError: true } as Record<string, unknown>
  )
  return {
    ...response.data,
    url: response.data.url || (response as unknown as { url?: string }).url,
  }
}

export async function reviewInvoiceApplication(
  applicationId: number,
  request: ReviewInvoiceApplicationRequest
): Promise<InvoiceApiResponse<null>> {
  const response = await api.post(
    `/api/invoice/admin/applications/${applicationId}/review`,
    request
  )
  return response.data
}

export async function deleteInvoiceApplication(
  applicationId: number
): Promise<InvoiceApiResponse<null>> {
  const response = await api.delete(
    `/api/invoice/admin/applications/${applicationId}`
  )
  return response.data
}

export async function uploadInvoiceFile(
  applicationId: number,
  file: File
): Promise<InvoiceApiResponse<null>> {
  const body = new FormData()
  body.append('file', file)
  const response = await api.post(
    `/api/invoice/admin/applications/${applicationId}/file`,
    body
  )
  return response.data
}

export async function getOwnInvoiceFile(applicationId: number): Promise<Blob> {
  const response = await api.get(
    `/api/invoice/applications/${applicationId}/file`,
    { responseType: 'blob' }
  )
  return response.data
}

export async function getAdminInvoiceFile(
  applicationId: number
): Promise<Blob> {
  const response = await api.get(
    `/api/invoice/admin/applications/${applicationId}/file`,
    {
      responseType: 'blob',
    }
  )
  return response.data
}

export async function sendOwnInvoiceEmail(
  applicationId: number
): Promise<InvoiceApiResponse<InvoiceApplication>> {
  const response = await api.post(
    `/api/invoice/applications/${applicationId}/send`
  )
  return response.data
}

export async function sendAdminInvoiceEmail(
  applicationId: number
): Promise<InvoiceApiResponse<InvoiceApplication>> {
  const response = await api.post(
    `/api/invoice/admin/applications/${applicationId}/send`
  )
  return response.data
}
