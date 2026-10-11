import type { Language } from "@/lib/i18n";

// formatDateTime 按界面语言显示时间；传入 timeZone 时按该时区显示，否则使用浏览器时区。
export function formatDateTime(value: string, language: Language, timeZone?: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  try {
    return date.toLocaleString(language, { hour12: false, timeZone: timeZone || undefined });
  } catch {
    return date.toLocaleString(language, { hour12: false });
  }
}
