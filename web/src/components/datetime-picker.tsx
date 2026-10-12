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
import Calendar03Icon from "@hugeicons/core-free-icons/Calendar03Icon";
import Cancel01Icon from "@hugeicons/core-free-icons/Cancel01Icon";
import Clock01Icon from "@hugeicons/core-free-icons/Clock01Icon";
import { HugeiconsIcon } from "@hugeicons/react";
import * as React from "react";
import { enUS, fr, ja, ru, vi, zhCN, zhTW } from "react-day-picker/locale";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import dayjs from "@/lib/dayjs";
import { cn } from "@/lib/utils";

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  "zh-CN": zhCN,
  "zh-TW": zhTW,
  zhCN,
  zhTW,
  fr,
  ru,
  ja,
  vi,
} as const;

function resolveCalendarLocale(language = "zh") {
  if (language in calendarLocales) {
    return calendarLocales[language as keyof typeof calendarLocales];
  }
  if (language.startsWith("zh-TW")) return zhTW;
  if (language.startsWith("zh")) return zhCN;
  if (language.startsWith("fr")) return fr;
  if (language.startsWith("ja")) return ja;
  if (language.startsWith("ru")) return ru;
  if (language.startsWith("vi")) return vi;
  return enUS;
}

interface DateTimePickerProps {
  value?: Date;
  onChange?: (date: Date | undefined) => void;
  placeholder?: string;
  className?: string;
  minDate?: Date;
  dateOnly?: boolean;
}

export function DateTimePicker({
  value,
  onChange,
  placeholder,
  className,
  minDate,
  dateOnly = false,
}: DateTimePickerProps) {
  const { t, i18n } = useTranslation();
  const placeholderText = placeholder ?? t("Select date");
  const calendarLocale = resolveCalendarLocale(i18n.language);
  const currentYear = new Date().getFullYear();
  const minSelectableDate = minDate
    ? new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate())
    : undefined;
  const [open, setOpen] = React.useState(false);
  const [date, setDate] = React.useState<Date | undefined>(value);
  const [month, setMonth] = React.useState<Date | undefined>(value);
  const [time, setTime] = React.useState<string>("00:00:00");
  const timeInputId = React.useId();

  React.useEffect(() => {
    setDate(value);
    setMonth(value);
    if (value) {
      setTime(dayjs(value).format("HH:mm:ss"));
    } else {
      setTime("00:00:00");
    }
  }, [value]);

  const handleDateSelect = (selectedDate: Date | undefined) => {
    if (selectedDate) {
      const [hours, minutes, seconds = 0] =
        /^(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$/.test(time)
          ? time.split(":").map(Number)
          : [0, 0];
      const newDate = new Date(selectedDate);
      newDate.setHours(hours, minutes, seconds, 0);
      if (minDate && newDate < minDate) {
        const nextTime = dayjs(minDate).format("HH:mm:ss");
        setTime(nextTime);
        setDate(minDate);
        setMonth(minDate);
        onChange?.(minDate);
        if (dateOnly) setOpen(false);
        return;
      }
      setDate(newDate);
      setMonth(newDate);
      onChange?.(newDate);
      if (dateOnly) setOpen(false);
    } else {
      setDate(undefined);
      setMonth(undefined);
      onChange?.(undefined);
    }
  };

  const handleTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newTime = e.target.value;
    // The browser reports an emptied or half-cleared field as ''. Keep the
    // current time: the field shows it again and no invalid date goes out.
    if (!newTime) return;
    setTime(newTime);
    if (!/^(?:[01]\d|2[0-3]):[0-5]\d(?::[0-5]\d)?$/.test(newTime)) return;

    if (date) {
      // A browser may omit :00 seconds from the value.
      const [hours, minutes, seconds = 0] = newTime.split(":").map(Number);
      const newDate = new Date(date);
      newDate.setHours(hours, minutes, seconds, 0);
      if (minDate && newDate < minDate) {
        const nextTime = dayjs(minDate).format("HH:mm:ss");
        setTime(nextTime);
        setDate(minDate);
        onChange?.(minDate);
        return;
      }
      setDate(newDate);
      onChange?.(newDate);
    }
  };

  const handleClear = () => {
    setDate(undefined);
    setMonth(undefined);
    setTime("00:00:00");
    onChange?.(undefined);
  };

  return (
    <div className={cn("flex gap-2", className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              variant="outline"
              className={cn(
                "min-w-0 flex-1 justify-between font-normal",
                !date && "text-muted-foreground",
              )}
            />
          }
        >
          <span className="flex min-w-0 items-center gap-1.5 truncate">
            <HugeiconsIcon
              icon={Calendar03Icon}
              data-icon="inline-start"
              strokeWidth={2}
            />
            <span className="truncate">
              {date
                ? dayjs(date).format(
                    dateOnly ? "YYYY-MM-DD" : "YYYY-MM-DD HH:mm:ss",
                  )
                : placeholderText}
            </span>
          </span>
        </PopoverTrigger>
        <PopoverContent className="w-auto overflow-hidden p-0" align="start">
          <Calendar
            mode="single"
            selected={date}
            month={month}
            onMonthChange={setMonth}
            captionLayout="dropdown"
            onSelect={handleDateSelect}
            locale={calendarLocale}
            disabled={
              minSelectableDate ? { before: minSelectableDate } : undefined
            }
            startMonth={new Date(currentYear - 100, 0)}
            endMonth={new Date(currentYear + 100, 11)}
          />
          {!dateOnly && (
            <div className="border-border flex items-end gap-2 border-t p-3">
              <Field className="gap-1.5">
                <FieldLabel
                  htmlFor={timeInputId}
                  className="text-muted-foreground text-xs"
                >
                  <HugeiconsIcon
                    icon={Clock01Icon}
                    data-icon="inline-start"
                    strokeWidth={2}
                  />
                  {t("Time")}
                </FieldLabel>
                <Input
                  id={timeInputId}
                  type="time"
                  step={1}
                  value={time}
                  onChange={handleTimeChange}
                  className={cn(
                    "w-32 appearance-none tabular-nums",
                    "[&::-webkit-calendar-picker-indicator]:hidden",
                    "[&::-webkit-calendar-picker-indicator]:appearance-none",
                  )}
                />
              </Field>
              <Button type="button" onClick={() => setOpen(false)}>
                {t("Done")}
              </Button>
            </div>
          )}
        </PopoverContent>
      </Popover>
      {date && (
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={handleClear}
          className="shrink-0"
          aria-label={t("Clear")}
        >
          <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} />
        </Button>
      )}
    </div>
  );
}
