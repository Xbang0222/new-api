/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { Controller, type UseFormReturn } from 'react-hook-form'
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
import { useSystemConfig } from '@/hooks/use-system-config'

import type { InvoiceRuleFormValues } from './lib/invoice-rule-form'

type InvoiceRulesFormProps = {
  form: UseFormReturn<InvoiceRuleFormValues>
  onSubmit: (values: InvoiceRuleFormValues) => void
}

export function InvoiceRulesForm(props: InvoiceRulesFormProps) {
  const { t } = useTranslation()
  const { currency } = useSystemConfig()
  let invoiceCurrency = currency.quotaDisplayType
  if (invoiceCurrency === 'TOKENS') invoiceCurrency = 'USD'
  const currencyLabel =
    invoiceCurrency === 'CUSTOM'
      ? currency.customCurrencySymbol
      : invoiceCurrency

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Invoice rules')}</CardTitle>
        <CardDescription>
          {t(
            'Configure the invoice item, minimum amount, and fee deducted from the user balance when applying.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={props.form.handleSubmit(props.onSubmit)}>
          <FieldGroup>
            <Controller
              control={props.form.control}
              name='enabled'
              render={({ field }) => (
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
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </Field>
              )}
            />

            <FieldGroup className='grid gap-5 sm:grid-cols-2'>
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
                  maxLength={255}
                  aria-invalid={Boolean(
                    props.form.formState.errors.invoiceItemName
                  )}
                  {...props.form.register('invoiceItemName')}
                />
                <FieldDescription>
                  {t('Use the actual service name shown on the invoice.')}
                </FieldDescription>
                <FieldError
                  errors={[props.form.formState.errors.invoiceItemName]}
                />
              </Field>

              <Field>
                <FieldLabel htmlFor='invoice-currency'>
                  {t('Currency')}
                </FieldLabel>
                <Input id='invoice-currency' value={currencyLabel} readOnly />
                <FieldDescription>
                  {t(
                    'Invoice currency follows the system currency settings. Token display uses USD.'
                  )}
                </FieldDescription>
              </Field>

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
                  aria-invalid={Boolean(
                    props.form.formState.errors.minimumAmount
                  )}
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
                  {t('Invoice fee rate (%)')}
                </FieldLabel>
                <Input
                  id='invoice-fee-rate'
                  type='number'
                  min='0'
                  max='100'
                  step='0.01'
                  aria-invalid={Boolean(
                    props.form.formState.errors.feeRatePercent
                  )}
                  {...props.form.register('feeRatePercent', {
                    valueAsNumber: true,
                  })}
                />
                <FieldDescription>
                  {t(
                    'The fee is deducted from the user balance when applying and refunded if the application is rejected or canceled.'
                  )}
                </FieldDescription>
                <FieldError
                  errors={[props.form.formState.errors.feeRatePercent]}
                />
              </Field>
            </FieldGroup>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
