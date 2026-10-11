import { Summary, SummaryFrequency } from "@/lib/types";

/**
 * summaryDateText 显示摘要覆盖的日期；每周和每月摘要显示完整范围。
 * 文本节点的英文由 i18n 动态翻译；属性等不经过翻译的位置传入 language。
 */
export function summaryDateText(item: Pick<Summary, "summaryDate" | "periodEnd">, language?: string) {
  if (!item.periodEnd || item.periodEnd === item.summaryDate) {
    return item.summaryDate;
  }
  return `${item.summaryDate}${language === "en" ? " to " : " 至 "}${item.periodEnd}`;
}

/**
 * periodPending 判断每周或每月摘要是否在周期结束前生成；这类摘要只是部分内容，周期结束后重新生成再发送。
 * timezone 为摘要时区，与后端判断一致。
 */
export function periodPending(
  item: Pick<Summary, "period" | "periodEnd" | "generatedAt" | "summaryDate">,
  timezone: string,
) {
  if (!item.period || item.period === "day") {
    return false;
  }
  // 与后端一致：缺少生成时间视为未就绪，缺少周期结束日期时按摘要日期判断。
  const generatedDate = item.generatedAt ? dateInTimezone(new Date(item.generatedAt), timezone) : "";
  return generatedDate === "" || generatedDate <= (item.periodEnd || item.summaryDate);
}

/** dateInTimezone 返回某一时刻在指定时区的日期（YYYY-MM-DD）；时刻或时区无效时返回空字符串。 */
export function dateInTimezone(moment: Date, timezone: string) {
  if (Number.isNaN(moment.getTime())) {
    return "";
  }
  try {
    return new Intl.DateTimeFormat("en-CA", {
      timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit",
    }).format(moment);
  } catch {
    return "";
  }
}

export const summaryFrequencyOptions: Array<{ value: SummaryFrequency; label: string }> = [
  { value: "day", label: "每天" },
  { value: "week", label: "每周（周一至周日）" },
  { value: "month", label: "每月" },
];

/** frequencyLabel 返回频率的简短名称，用于切换提示。 */
export function frequencyLabel(frequency: SummaryFrequency | undefined) {
  if (frequency === "week") return "每周";
  if (frequency === "month") return "每月";
  return "每天";
}
