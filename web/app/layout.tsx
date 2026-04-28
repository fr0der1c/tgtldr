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

export default function RootLayout({
  children
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="zh-CN">
      <body>
        <I18nProvider>
          <ToastProvider>{children}</ToastProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
