"use client";

import { ChatMessageActivity } from "@/lib/types";
import { ChatActivityStrip } from "@/components/chat-activity-strip";
import { ChatDatePicker } from "@/components/chat-date-picker";

type ChatHistoryToolbarProps = {
  activity: ChatMessageActivity[];
  busy: boolean;
  currentDate: string;
  nextDate?: string;
  previousDate?: string;
  total?: number;
  filtersApplied: boolean;
  hasMessageFilters: boolean;
  onDateChange: (date: string) => void;
  onFiltersChange: (enabled: boolean) => void;
};

export function ChatHistoryToolbar(props: ChatHistoryToolbarProps) {
  return (
    <div className="chat-history-toolbar">
      <div className="chat-history-date-row">
        <div className="chat-date-pager">
          <button
            aria-label="上一有消息日"
            className="chat-date-arrow"
            disabled={props.busy || !props.previousDate}
            onClick={() => props.previousDate && props.onDateChange(props.previousDate)}
            title="上一有消息日"
            type="button"
          >
            <ChevronIcon direction="left" />
          </button>
          <ChatDatePicker
            activity={props.activity}
            busy={props.busy}
            currentDate={props.currentDate}
            onDateChange={props.onDateChange}
            total={props.total}
          />
          <button
            aria-label="下一有消息日"
            className="chat-date-arrow"
            disabled={props.busy || !props.nextDate}
            onClick={() => props.nextDate && props.onDateChange(props.nextDate)}
            title="下一有消息日"
            type="button"
          >
            <ChevronIcon direction="right" />
          </button>
        </div>

        <div className="chat-history-activity">
          <div className="chat-history-activity-labels">
            <span>最近 30 天</span>
            <small>{activityRange(props.activity)}</small>
          </div>
          <ChatActivityStrip
            activity={props.activity}
            currentDate={props.currentDate}
            onSelectDate={props.onDateChange}
          />
        </div>

        <label
          className={`chat-filter-toggle${props.filtersApplied ? " active" : ""}`}
          title={props.hasMessageFilters ? "过滤发言人和关键词" : "该群尚未配置过滤发言人或关键词"}
        >
          <input
            checked={props.filtersApplied}
            disabled={props.busy || !props.hasMessageFilters}
            onChange={(event) => props.onFiltersChange(event.target.checked)}
            type="checkbox"
          />
          <span aria-hidden="true"><i /></span>
          <b>过滤发言人和关键词</b>
        </label>
      </div>
    </div>
  );
}

function activityRange(activity: ChatMessageActivity[]) {
  if (activity.length === 0) return "";
  return `${activity[0].date.slice(5)} – ${activity[activity.length - 1].date.slice(5)}`;
}

function ChevronIcon({ direction }: { direction: "left" | "right" }) {
  const path = direction === "left" ? "m14.5 6-6 6 6 6" : "m9.5 6 6 6-6 6";
  return <svg aria-hidden="true" fill="none" viewBox="0 0 24 24"><path d={path} stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.9" /></svg>;
}
