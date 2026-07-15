import Link from "next/link";
import { ChatMessageActivity } from "@/lib/types";

type ChatActivityStripProps = {
  activity: ChatMessageActivity[];
  currentDate?: string;
  hrefForDate?: (date: string) => string;
  onSelectDate?: (date: string) => void;
};

export function ChatActivityStrip({
  activity,
  currentDate,
  hrefForDate,
  onSelectDate,
}: ChatActivityStripProps) {
  if (activity.length === 0) {
    return null;
  }

  return (
    <div aria-label="最近 30 天消息" className="chat-activity-strip" role="list">
      {activity.map((item) => {
        const active = item.messageCount > 0;
        const selected = item.date === currentDate;
        const className = [
          "chat-activity-day",
          active ? "active" : "",
          selected ? "selected" : "",
        ].filter(Boolean).join(" ");
        const label = `${item.date}：${item.messageCount} 条消息`;

        if (active && hrefForDate) {
          return (
            <Link
              aria-label={label}
              className={className}
              data-tooltip={label}
              href={hrefForDate(item.date)}
              key={item.date}
              role="listitem"
            />
          );
        }
        if (active && onSelectDate) {
          return (
            <button
              aria-label={label}
              className={className}
              data-tooltip={label}
              key={item.date}
              onClick={() => onSelectDate(item.date)}
              role="listitem"
              type="button"
            />
          );
        }
        return (
          <span
            aria-label={label}
            className={className}
            data-tooltip={label}
            key={item.date}
            role="listitem"
          />
        );
      })}
    </div>
  );
}
