/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import type { InvoiceRuleFormValues } from './lib/invoice-rule-form'

type InvoiceRulesFormProps = {
  form: UseFormReturn<InvoiceRuleFormValues>
  onSubmit: (values: InvoiceRuleFormValues) => void
}

export function InvoiceRulesForm(props: InvoiceRulesFormProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Invoice rules')}</CardTitle>
        <CardDescription>
          {t(
            'Invoice currency and quota conversion follow the current wallet settlement settings.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={props.form.handleSubmit(props.onSubmit)}>
          <FieldGroup>
            <Field orientation='horizontal'>
              <div className='flex flex-1 flex-col gap-0.5'>
                <FieldLabel htmlFor='invoice-enabled'>
                  {t('Enable invoice applications')}
                </FieldLabel>
                <FieldDescription>
                  {t(
                    'Allow users to create applications from eligible paid orders.'
                  )}
                </FieldDescription>
              </div>
              <Switch
                id='invoice-enabled'
                checked={props.form.watch('enabled')}
                onCheckedChange={(value) =>
                  props.form.setValue('enabled', value)
                }
              />
            </Field>
            <Field
              data-invalid={Boolean(
                props.form.formState.errors.invoiceItemName
              )}
            >
              <FieldLabel htmlFor='invoice-item-name'>
                {t('Invoice item name')}
              </FieldLabel>
              <Input
                id='invoice-item-name'
                {...props.form.register('invoiceItemName')}
              />
              <FieldError
                errors={[props.form.formState.errors.invoiceItemName]}
              />
            </Field>
            <div className='grid gap-5 sm:grid-cols-2'>
              <Field
                data-invalid={Boolean(
                  props.form.formState.errors.minimumAmount
                )}
              >
                <FieldLabel htmlFor='invoice-minimum-amount'>
                  {t('Minimum invoice amount')}
                </FieldLabel>
                <Input
                  id='invoice-minimum-amount'
                  type='number'
                  min='0'
                  step='0.01'
                  {...props.form.register('minimumAmount', {
                    valueAsNumber: true,
                  })}
                />
                <FieldError
                  errors={[props.form.formState.errors.minimumAmount]}
                />
              </Field>
              <Field
                data-invalid={Boolean(
                  props.form.formState.errors.feeRatePercent
                )}
              >
                <FieldLabel htmlFor='invoice-fee-rate'>
                  {t('Invoice fee rate')}
                </FieldLabel>
                <Input
                  id='invoice-fee-rate'
                  type='number'
                  min='0'
                  max='100'
                  step='0.01'
                  {...props.form.register('feeRatePercent', {
                    valueAsNumber: true,
                  })}
                />
                <FieldDescription>
                  {t(
                    'The fee is deducted from the user balance when the application is submitted.'
                  )}
                </FieldDescription>
                <FieldError
                  errors={[props.form.formState.errors.feeRatePercent]}
                />
              </Field>
            </div>
          </FieldGroup>
          <button type='submit' className='hidden' aria-hidden='true' />
        </form>
      </CardContent>
    </Card>
  )
}
