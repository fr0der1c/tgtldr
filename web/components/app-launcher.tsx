"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Bootstrap } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

function resolveEntryRoute(bootstrap: Bootstrap) {
  if (!bootstrap.passwordConfigured) {
    return "/setup";
  }
  if (!bootstrap.authenticated) {
    return "/login";
  }
  if (!bootstrap.settingsConfigured) {
    return "/setup";
  }
  if (!bootstrap.telegramAuthorized) {
    return "/setup";
  }
  return "/dashboard";
}

export function AppLauncher() {
  const router = useRouter();
  const { setLanguage } = useI18n();
  const [message, setMessage] = useState("正在检查当前状态...");

  useEffect(() => {
    let cancelled = false;

    async function launch() {
      try {
        const bootstrap = await api.bootstrap();
        if (cancelled) {
          return;
        }
        setLanguage(bootstrap.language);

        const target = resolveEntryRoute(bootstrap);
        if (target === "/setup") {
          setMessage("正在进入首次配置...");
        } else if (target === "/login") {
          setMessage("正在进入登录...");
        } else {
          setMessage("正在进入后台...");
        }
        router.replace(target);
      } catch {
        if (cancelled) {
          return;
        }
        setMessage("无法读取当前状态，正在进入首次配置...");
        router.replace("/setup");
      }
    }

    void launch();

    return () => {
      cancelled = true;
    };
  }, [router, setLanguage]);

  return (
    <main aria-busy="true" className="app-launch">
      <div className="app-launch-shell">
        <p aria-hidden="true" className="app-launch-mark">
          TGTLDR
        </p>
        <p aria-live="polite" className="sr-only" role="status">
          {message}
        </p>
      </div>
    </main>
  );
}
