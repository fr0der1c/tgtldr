"use client";

import * as Popover from "@radix-ui/react-popover";
import { CSSProperties, useEffect, useMemo, useState } from "react";
import { DayPicker } from "react-day-picker";
import { enUS, zhCN } from "react-day-picker/locale";
import { ChatMessageActivity } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

export function ChatDatePicker({
  activity,
  busy,
  currentDate,
  onDateChange,
  total,
}: {
  activity: ChatMessageActivity[];
  busy: boolean;
  currentDate: string;
  onDateChange: (date: string) => void;
  total?: number;
}) {
  const { language } = useI18n();
  const selected = useMemo(() => parseCalendarDate(currentDate), [currentDate]);
  const activeDates = useMemo(
    () => activity.filter((item) => item.messageCount > 0).map((item) => parseCalendarDate(item.date)),
    [activity],
  );
  const [month, setMonth] = useState(selected);
  const [open, setOpen] = useState(false);

  useEffect(() => setMonth(selected), [selected]);
  useEffect(() => {
    if (busy) setOpen(false);
  }, [busy]);

  function selectDate(date: Date | undefined) {
    if (!date) return;
    setOpen(false);
    onDateChange(formatCalendarValue(date));
  }

  return (
    <Popover.Root onOpenChange={setOpen} open={open}>
      <Popover.Trigger asChild>
        <button
          aria-label="打开日期选择器"
          className="chat-date-current"
          disabled={busy}
          type="button"
        >
          <span>{formatCalendarDate(currentDate, language)}</span>
          {total === undefined ? null : <small>{total} 条消息</small>}
          <CalendarIcon />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="center"
          className="chat-date-popover"
          collisionPadding={12}
          sideOffset={10}
        >
          <DayPicker
            aria-label="选择聊天记录日期"
            autoFocus
            className="chat-date-calendar"
            components={{ Chevron: CalendarChevron }}
            locale={language === "zh-CN" ? zhCN : enUS}
            mode="single"
            modifiers={{ hasMessages: activeDates }}
            modifiersClassNames={{ hasMessages: "chat-calendar-has-messages" }}
            month={month}
            navLayout="around"
            onMonthChange={setMonth}
            onSelect={selectDate}
            required
            selected={selected}
            weekStartsOn={1}
          />
          <Popover.Arrow className="chat-date-popover-arrow" height={7} width={12} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function parseCalendarDate(value: string) {
  const [year, month, day] = value.split("-").map(Number);
  return new Date(year, month - 1, day, 12);
}

function formatCalendarValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatCalendarDate(date: string, language: "zh-CN" | "en") {
  const parsed = parseCalendarDate(date);
  const datePart = new Intl.DateTimeFormat(language, {
    year: "numeric", month: "2-digit", day: "2-digit",
  }).format(parsed);
  const weekday = new Intl.DateTimeFormat(language, { weekday: "short" }).format(parsed);
  return `${datePart} ${weekday}`;
}

function CalendarIcon() {
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><rect height="15" rx="2" stroke="currentColor" strokeWidth="1.7" width="17" x="3.5" y="5.5" /><path d="M8 3.5v4M16 3.5v4M3.5 10h17" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" /></svg>;
}

function CalendarChevron({
  className,
  orientation = "right",
  style,
}: {
  className?: string;
  orientation?: "up" | "down" | "left" | "right";
  style?: CSSProperties;
}) {
  const paths = {
    down: "m6 9.5 6 6 6-6",
    left: "m14.5 6-6 6 6 6",
    right: "m9.5 6 6 6-6 6",
    up: "m6 14.5 6-6 6 6",
  };
  return <svg aria-hidden="true" className={className} fill="none" style={style} viewBox="0 0 24 24"><path d={paths[orientation]} stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" /></svg>;
}
