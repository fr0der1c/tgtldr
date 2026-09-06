import type { Summary } from "./types";

/** 历史摘要缺少请求模型时保留原标签，不从当前设置推测历史请求。 */
export function summaryModelLabel(summary: Pick<Summary, "model" | "requestedModel" | "returnedModel">, language: string): string {
  const requested = summary.requestedModel?.trim();
  const returned = summary.returnedModel?.trim();
  if (!requested) return summary.model || (language === "en" ? "Model not recorded" : "未记录模型");
  if (!returned || requested === returned) return requested;
  return language === "en"
    ? `${requested} (returned: ${returned})`
    : `${requested}（实际返回：${returned}）`;
}
