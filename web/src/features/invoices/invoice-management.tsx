/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'

import {
  deleteInvoiceApplication,
  getAdminInvoiceApplications,
  getAdminInvoiceFile,
  reviewInvoiceApplication,
  sendAdminInvoiceEmail,
  uploadInvoiceFile,
} from './api'
import { InvoiceApplicationsList } from './components/invoice-applications-list'
import { InvoiceDetailsDialog } from './components/invoice-details-dialog'
import type {
  InvoiceApplication,
  InvoiceStatus,
  ReviewInvoiceApplicationRequest,
} from './types'

export function InvoiceManagement() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [status, setStatus] = useState<InvoiceStatus>()
  const [keyword, setKeyword] = useState('')
  const [detailsTarget, setDetailsTarget] = useState<InvoiceApplication | null>(
    null
  )
  const [deleteTarget, setDeleteTarget] = useState<InvoiceApplication | null>(
    null
  )
  const [fileActionPending, setFileActionPending] = useState(false)

  const applicationsQuery = useQuery({
    queryKey: ['invoices', 'admin', page, pageSize, status, keyword],
    queryFn: () =>
      getAdminInvoiceApplications(page, pageSize, {
        status,
        keyword: keyword.trim() || undefined,
      }),
    placeholderData: (previousData) => previousData,
  })
  const refreshApplications = () =>
    queryClient.invalidateQueries({ queryKey: ['invoices', 'admin'] })

  const reviewMutation = useMutation({
    mutationFn: async (request: ReviewInvoiceApplicationRequest) => {
      if (!detailsTarget) {
        throw new Error(t('Invoice application is unavailable.'))
      }
      const response = await reviewInvoiceApplication(detailsTarget.id, request)
      if (!response.success) throw new Error(response.message)
      return response
    },
    onSuccess: async () => {
      toast.success(t('Invoice review completed.'))
      setDetailsTarget(null)
      await refreshApplications()
    },
    onError: () => toast.error(t('Failed to review invoice application.')),
  })

  const deleteMutation = useMutation({
    mutationFn: async (applicationId: number) => {
      const response = await deleteInvoiceApplication(applicationId)
      if (!response.success) throw new Error(response.message)
      return response
    },
    onSuccess: async () => {
      toast.success(t('Deleted successfully'))
      setDetailsTarget(null)
      setDeleteTarget(null)
      await refreshApplications()
    },
    onError: () => toast.error(t('Delete failed')),
  })

  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      if (!detailsTarget) {
        throw new Error(t('Invoice application is unavailable.'))
      }
      const response = await uploadInvoiceFile(detailsTarget.id, file)
      if (!response.success) throw new Error(response.message)
      return response
    },
    onSuccess: async () => {
      toast.success(t('Invoice uploaded and sent.'))
      setDetailsTarget(null)
      await refreshApplications()
    },
    onError: async () => {
      toast.error(
        t(
          'Invoice upload or email delivery failed. Refresh the list and try again.'
        )
      )
      setDetailsTarget(null)
      await refreshApplications()
    },
  })

  const sendMutation = useMutation({
    mutationFn: async (application: InvoiceApplication) => {
      const response = await sendAdminInvoiceEmail(application.id)
      if (!response.success) throw new Error(response.message)
      return response
    },
    onSuccess: async () => {
      toast.success(t('Invoice email sent.'))
      setDetailsTarget(null)
      await refreshApplications()
    },
    onError: () => toast.error(t('Failed to send invoice email.')),
  })

  const handleViewInvoiceFile = async () => {
    if (!detailsTarget) return
    const previewWindow = window.open('about:blank', '_blank')
    if (!previewWindow) {
      toast.error(t('Failed to download invoice.'))
      return
    }
    previewWindow.opener = null
    setFileActionPending(true)
    try {
      const blob = await getAdminInvoiceFile(detailsTarget.id)
      const objectUrl = URL.createObjectURL(blob)
      previewWindow.location.href = objectUrl
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000)
    } catch {
      previewWindow.close()
      toast.error(t('Failed to download invoice.'))
    } finally {
      setFileActionPending(false)
    }
  }

  const applications = applicationsQuery.data?.data
  const busy =
    reviewMutation.isPending ||
    uploadMutation.isPending ||
    sendMutation.isPending ||
    deleteMutation.isPending ||
    fileActionPending

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Invoice Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <InvoiceApplicationsList
            applications={applications?.items ?? []}
            loading={applicationsQuery.isLoading}
            fetching={applicationsQuery.isFetching}
            page={page}
            pageSize={pageSize}
            total={applications?.total ?? 0}
            keyword={keyword}
            status={status}
            isAdmin
            sendingId={
              sendMutation.isPending
                ? (sendMutation.variables?.id ?? null)
                : null
            }
            deletingId={
              deleteMutation.isPending ? deleteMutation.variables : null
            }
            refreshing={applicationsQuery.isFetching}
            onPageChange={setPage}
            onPageSizeChange={setPageSize}
            onKeywordChange={setKeyword}
            onStatusChange={setStatus}
            onView={setDetailsTarget}
            onSend={(application) => sendMutation.mutate(application)}
            onDelete={setDeleteTarget}
            onRefresh={() => void applicationsQuery.refetch()}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <InvoiceDetailsDialog
        application={detailsTarget}
        isAdmin
        busy={busy}
        onOpenChange={(open) => !open && setDetailsTarget(null)}
        onReview={(request) => reviewMutation.mutate(request)}
        onCancel={() => undefined}
        onUpload={(file) => uploadMutation.mutate(file)}
        onViewFile={() => void handleViewInvoiceFile()}
        onSend={() => detailsTarget && sendMutation.mutate(detailsTarget)}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Confirm delete')}
        desc={t('This action cannot be undone.')}
        destructive
        confirmText={t('Delete')}
        isLoading={deleteMutation.isPending}
        handleConfirm={() =>
          deleteTarget && deleteMutation.mutate(deleteTarget.id)
        }
      />
    </>
  )
}
