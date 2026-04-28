"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { Button, Card, Field, Input } from "@/components/ui";
import { Bootstrap } from "@/lib/types";
import { useI18n } from "@/lib/i18n";

export function LoginForm() {
  const router = useRouter();
  const { setLanguage } = useI18n();
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await api.bootstrap();
        if (cancelled) {
          return;
        }
        setLanguage(data.language);
        setBootstrap(data);
        if (!data.passwordConfigured) {
          router.replace("/setup");
          return;
        }
        if (data.authenticated) {
          router.replace(
            data.settingsConfigured && data.telegramAuthorized
              ? "/dashboard/chats"
              : "/setup",
          );
        }
      } catch (err) {
        if (cancelled) {
          return;
        }
        setError(err instanceof Error ? err.message : "无法读取当前状态。");
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [router, setLanguage]);

  async function submit() {
    if (submitting) {
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await api.login(password);
      const data = await api.bootstrap();
      router.replace(
        data.settingsConfigured && data.telegramAuthorized
          ? "/dashboard/chats"
          : "/setup",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败。");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="page-shell">
      <div className="login-shell">
        <Card
          title="访问登录"
          description="请输入你在首次设置中配置的访问密码。"
        >
          <div className="form-grid compact">
            <Field label="访问密码">
              <Input
                autoComplete="current-password"
                onChange={(event) => setPassword(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void submit();
                  }
                }}
                type="password"
                value={password}
              />
            </Field>
          </div>
          <div className="setup-actions">
            <Button
              disabled={
                !bootstrap?.passwordConfigured ||
                password.trim() === "" ||
                submitting
              }
              onClick={() => void submit()}
              type="button"
            >
              登录
            </Button>
          </div>
          {error ? (
            <p aria-live="assertive" className="notice bad" role="alert">
              {error}
            </p>
          ) : null}
        </Card>
      </div>
    </main>
  );
}
