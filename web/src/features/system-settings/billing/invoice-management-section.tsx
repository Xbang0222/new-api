/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { updateSystemOption } from '../api'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { InvoiceRulesForm } from './invoice-rules-form'
import {
  createInvoiceRuleSchema,
  type InvoiceRuleFormValues,
} from './lib/invoice-rule-form'

export type InvoiceRuleDefaults = InvoiceRuleFormValues

type InvoiceManagementSectionProps = {
  defaultValues: InvoiceRuleDefaults
}

export function InvoiceManagementSection(props: InvoiceManagementSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<InvoiceRuleFormValues>({
    resolver: zodResolver(
      createInvoiceRuleSchema(t)
    ) as unknown as Resolver<InvoiceRuleFormValues>,
    defaultValues: props.defaultValues,
    values: props.defaultValues,
  })

  const saveRulesMutation = useMutation({
    mutationFn: async (values: InvoiceRuleFormValues) => {
      const updates = [
        ['invoice_setting.enabled', values.enabled],
        ['invoice_setting.minimum_amount', values.minimumAmount],
        ['invoice_setting.invoice_item_name', values.invoiceItemName],
        [
          'invoice_setting.fee_rate_basis_points',
          Math.round(values.feeRatePercent * 100),
        ],
      ] as const
      const responses = await Promise.all(
        updates.map(([key, value]) => updateSystemOption({ key, value }))
      )
      const failed = responses.find((response) => !response.success)
      if (failed) {
        throw new Error(failed.message || t('Failed to update setting'))
      }
    },
    onSuccess: async () => {
      toast.success(t('Invoice rules saved.'))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['system-options'] }),
        queryClient.invalidateQueries({ queryKey: ['invoices', 'config'] }),
      ])
    },
    onError: () => toast.error(t('Failed to save invoice settings.')),
  })

  return (
    <div className='flex flex-col gap-4'>
      <SettingsPageFormActions
        onSave={form.handleSubmit((values) => saveRulesMutation.mutate(values))}
        onReset={() => form.reset(props.defaultValues)}
        isSaving={saveRulesMutation.isPending}
        saveLabel='Save invoice rules'
      />
      <InvoiceRulesForm
        form={form}
        onSubmit={(values) => saveRulesMutation.mutate(values)}
      />
    </div>
  )
}
