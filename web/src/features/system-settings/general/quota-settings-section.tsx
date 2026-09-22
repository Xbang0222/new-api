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
import { zodResolver } from '@hookform/resolvers/zod'
import type { ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const createQuotaSchema = (t: (key: string) => string) =>
  z
    .object({
      QuotaForNewUser: z.coerce.number().min(0),
      PreConsumedQuota: z.coerce.number().min(0),
      QuotaForInviter: z.coerce.number().min(0),
      QuotaForInvitee: z.coerce.number().min(0),
      InviterRewardType: z.enum(['disabled', 'fixed', 'percentage']),
      InviterRewardValue: z.coerce.number().min(0),
      MinAffTransferQuota: z.coerce.number().min(0),
      TopUpLink: z.string(),
      quota_setting: z.object({
        enable_free_model_pre_consume: z.boolean(),
        trust_quota_usd: z.preprocess(
          (value) => (value === '' ? undefined : value),
          z.coerce.number().min(0)
        ),
        pre_consume_multiplier: z.coerce.number().positive(),
      }),
    })
    .superRefine((data, ctx) => {
      if (
        data.InviterRewardType === 'percentage' &&
        data.InviterRewardValue > 100
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['InviterRewardValue'],
          message: t('Value must be between 0 and 100'),
        })
      }
    })

type QuotaFormValues = z.infer<ReturnType<typeof createQuotaSchema>>
type InviterRewardType = QuotaFormValues['InviterRewardType']
type QuotaInputValue = number | ''

function formatQuotaInputValue(value: QuotaInputValue): string {
  return formatQuota(value === '' ? 0 : value)
}

type QuotaSettingsSectionProps = {
  defaultValues: Partial<QuotaFormValues> &
    Pick<QuotaFormValues, 'QuotaForNewUser' | 'TopUpLink' | 'quota_setting'>
  complianceConfirmed?: boolean
}

export function QuotaSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: QuotaSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaSchema = createQuotaSchema(t)
  const rewardTypeDefault =
    defaultValues.InviterRewardType === 'fixed' ||
    defaultValues.InviterRewardType === 'percentage'
      ? defaultValues.InviterRewardType
      : 'disabled'
  const formDefaultValues: QuotaFormValues = {
    ...defaultValues,
    PreConsumedQuota: defaultValues.PreConsumedQuota ?? 0,
    QuotaForInviter: defaultValues.QuotaForInviter ?? 0,
    QuotaForInvitee: defaultValues.QuotaForInvitee ?? 0,
    InviterRewardType: rewardTypeDefault,
    InviterRewardValue:
      rewardTypeDefault === 'fixed'
        ? quotaUnitsToDollars(defaultValues.InviterRewardValue ?? 0)
        : (defaultValues.InviterRewardValue ?? 0),
    MinAffTransferQuota: quotaUnitsToDollars(
      defaultValues.MinAffTransferQuota ?? 0
    ),
  }
  const handleNumberChange =
    (onChange: (value: QuotaInputValue) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }
  const serializeRewardValue = (
    rewardType: InviterRewardType,
    rewardValue: number
  ) => {
    if (rewardType === 'fixed') {
      return parseQuotaFromDollars(rewardValue)
    }
    if (rewardType === 'percentage') {
      return Math.round(rewardValue)
    }
    return 0
  }
  const getRewardTypeLabel = (value: InviterRewardType) => {
    if (value === 'fixed') return t('Fixed quota')
    if (value === 'percentage') return t('Percentage of recharge')
    return t('Disabled')
  }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<QuotaFormValues>({
      resolver: zodResolver(quotaSchema) as Resolver<
        QuotaFormValues,
        unknown,
        QuotaFormValues
      >,
      defaultValues: formDefaultValues,
      onSubmit: async (data, changedFields) => {
        const serializedChanges: Record<string, string | number | boolean> = {
          ...(changedFields as Record<string, string | number | boolean>),
        }

        if ('InviterRewardType' in changedFields) {
          serializedChanges.InviterRewardType =
            data.InviterRewardType === 'disabled' ? '' : data.InviterRewardType
          if (data.InviterRewardType === 'disabled') {
            delete serializedChanges.InviterRewardValue
          } else {
            serializedChanges.InviterRewardValue = serializeRewardValue(
              data.InviterRewardType,
              data.InviterRewardValue
            )
          }
        }

        if (
          'InviterRewardValue' in changedFields &&
          !('InviterRewardType' in changedFields)
        ) {
          if (data.InviterRewardType === 'disabled') {
            delete serializedChanges.InviterRewardValue
          } else {
            serializedChanges.InviterRewardValue = serializeRewardValue(
              data.InviterRewardType,
              data.InviterRewardValue
            )
          }
        }

        if ('MinAffTransferQuota' in changedFields) {
          serializedChanges.MinAffTransferQuota = parseQuotaFromDollars(
            data.MinAffTransferQuota
          )
        }

        const priority =
          data.InviterRewardType === 'percentage'
            ? ['InviterRewardValue', 'InviterRewardType']
            : ['InviterRewardType', 'InviterRewardValue']
        const entries = Object.entries(serializedChanges).sort(
          ([left], [right]) => {
            const leftIndex = priority.indexOf(left)
            const rightIndex = priority.indexOf(right)
            if (leftIndex === -1 && rightIndex === -1) return 0
            if (leftIndex === -1) return 1
            if (rightIndex === -1) return -1
            return leftIndex - rightIndex
          }
        )

        for (const [key, value] of entries) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })
  const rewardType = form.watch('InviterRewardType')
  const rewardEnabled = rewardType !== 'disabled'

  return (
    <SettingsSection title={t('Quota Settings')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('New User Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Initial quota given to new users ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='PreConsumedQuota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Pre-Consumed Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota consumed before charging users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inviter Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Quota given to users who invite others ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitee Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota given to invited users ({{formattedQuota}})', {
                      formattedQuota: formatQuotaInputValue(field.value),
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <div className='bg-muted/20 flex min-w-0 flex-col gap-4 rounded-xl border p-4'>
                <div className='flex min-w-0 flex-col gap-1'>
                  <h3 className='text-sm font-medium'>
                    {t('Invitation Recharge Rebate')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {t('Configure rebates earned when invited users add funds')}
                  </p>
                </div>

                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='InviterRewardType'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Recharge Rebate Type')}</FormLabel>
                        <Select
                          value={field.value}
                          onValueChange={(value) => {
                            const nextValue = value as InviterRewardType
                            const previousValue = field.value
                            field.onChange(nextValue)
                            if (
                              nextValue !== 'disabled' &&
                              previousValue !== 'disabled' &&
                              (previousValue === 'fixed') !==
                                (nextValue === 'fixed')
                            ) {
                              form.setValue('InviterRewardValue', 0, {
                                shouldDirty: true,
                                shouldValidate: true,
                              })
                            }
                          }}
                        >
                          <FormControl>
                            <SelectTrigger className='w-full'>
                              <SelectValue>
                                {getRewardTypeLabel(field.value)}
                              </SelectValue>
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='disabled'>
                                {t('Disabled')}
                              </SelectItem>
                              <SelectItem value='fixed'>
                                {t('Fixed quota')}
                              </SelectItem>
                              <SelectItem value='percentage'>
                                {t('Percentage of recharge')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t(
                            'Choose how inviters are rewarded when invited users recharge'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='InviterRewardValue'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Recharge Rebate Value')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            max={rewardType === 'percentage' ? 100 : undefined}
                            step={rewardType === 'percentage' ? 1 : 0.01}
                            value={field.value ?? ''}
                            onChange={handleNumberChange(field.onChange)}
                            name={field.name}
                            onBlur={field.onBlur}
                            ref={field.ref}
                            disabled={!rewardEnabled}
                          />
                        </FormControl>
                        <FormDescription>
                          {rewardType === 'percentage'
                            ? t(
                                'Percentage of each invited recharge awarded as rebate'
                              )
                            : t(
                                'Fixed display amount awarded for each invited recharge'
                              )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='MinAffTransferQuota'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Minimum Transfer Threshold')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={0.01}
                            value={field.value ?? ''}
                            onChange={handleNumberChange(field.onChange)}
                            name={field.name}
                            onBlur={field.onBlur}
                            ref={field.ref}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Reward balance must reach this display amount before users can transfer it; 0 uses the default threshold'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>
              </div>
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='quota_setting.enable_free_model_pre_consume'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Pre-Consume for Free Models')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, zero-cost models also pre-consume quota before final settlement.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <FormField
              control={form.control}
              name='quota_setting.trust_quota_usd'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Wallet pre-consume bypass threshold (USD)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step='any'
                      value={field.value ?? ''}
                      onChange={field.onChange}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Skip pre-consumption when the wallet balance and limited API key balance both exceed this amount. Set to 0 to always pre-consume. Subscriptions and asynchronous tasks always reserve quota.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='quota_setting.pre_consume_multiplier'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Input pre-consume multiplier')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step='any'
                      value={field.value ?? ''}
                      onChange={field.onChange}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Reserve the estimated input cost multiplied by this value, without estimating output tokens. Defaults to 1; positive decimals such as 0.5 and 1.5 are supported. Final charges use actual usage. Per-request and task prices are unaffected.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='TopUpLink'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Top-Up Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/topup')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('External link for users to purchase quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
