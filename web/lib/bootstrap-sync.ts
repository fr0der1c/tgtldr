const BOOTSTRAP_REFRESH_EVENT = "tgtldr:bootstrap-refresh";

export function notifyBootstrapRefresh() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(BOOTSTRAP_REFRESH_EVENT));
}

export function onBootstrapRefresh(listener: () => void) {
  if (typeof window === "undefined") {
    return () => undefined;
  }

  const handler = () => listener();
  window.addEventListener(BOOTSTRAP_REFRESH_EVENT, handler);
  return () => window.removeEventListener(BOOTSTRAP_REFRESH_EVENT, handler);
}
