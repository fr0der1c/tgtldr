import "./globals.css";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { ToastProvider } from "@/components/toast";
import { I18nProvider } from "@/lib/i18n";

export const metadata: Metadata = {
  title: "TGTLDR",
  description: "Telegram 群组监听与每日摘要平台",
  icons: {
    icon: "/icon.png",
    apple: "/apple-icon.png"
  }
};

const languageBootScript = `
(() => {
  try {
    const match = document.cookie.match(/(?:^|; )tgtldr_language=([^;]+)/);
    const saved = match ? decodeURIComponent(match[1]) : "";
    const browser = navigator.language.toLowerCase().startsWith("en")
      ? "en"
      : "zh-CN";
    const language = saved === "en" || saved === "zh-CN" ? saved : browser;
    document.documentElement.lang = language;
    if (language === "en") {
      document.documentElement.classList.add("i18n-pending");
    }
  } catch {
  }
})();
`;

export default function RootLayout({
  children
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: languageBootScript }} />
      </head>
      <body>
        <I18nProvider>
          <ToastProvider>{children}</ToastProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
