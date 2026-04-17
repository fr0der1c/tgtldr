"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Bootstrap } from "@/lib/types";

function resolveEntryRoute(bootstrap: Bootstrap) {
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
  const [message, setMessage] = useState("正在检查当前状态...");

  useEffect(() => {
    let cancelled = false;

    async function launch() {
      try {
        const bootstrap = await api.bootstrap();
        if (cancelled) {
          return;
        }

        const target = resolveEntryRoute(bootstrap);
        setMessage(target === "/setup" ? "正在进入首次配置..." : "正在进入后台...");
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
  }, [router]);

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
