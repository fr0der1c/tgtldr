import { NextRequest } from "next/server";

const defaultAPIBaseURL = "http://app:8080";
const hopByHopHeaders = new Set([
  "connection",
  "content-length",
  "host",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);

function resolveInternalAPIBaseURL() {
  const raw = process.env.TGTLDR_INTERNAL_API_BASE_URL?.trim();
  if (raw) {
    return raw.replace(/\/+$/, "");
  }
  return defaultAPIBaseURL;
}

function buildTargetURL(req: NextRequest, path: string[]) {
  const target = new URL(`${resolveInternalAPIBaseURL()}/api/${path.join("/")}`);
  target.search = req.nextUrl.search;
  return target;
}

function copyRequestHeaders(req: NextRequest) {
  const headers = new Headers();
  req.headers.forEach((value, key) => {
    if (hopByHopHeaders.has(key.toLowerCase())) {
      return;
    }
    headers.set(key, value);
  });
  return headers;
}

async function readRequestBody(req: NextRequest) {
  if (req.method === "GET" || req.method === "HEAD") {
    return undefined;
  }
  const body = await req.arrayBuffer();
  if (body.byteLength === 0) {
    return undefined;
  }
  return body;
}

function copyResponseHeaders(source: Headers) {
  const headers = new Headers();
  source.forEach((value, key) => {
    if (key.toLowerCase() === "set-cookie") {
      return;
    }
    if (hopByHopHeaders.has(key.toLowerCase())) {
      return;
    }
    headers.append(key, value);
  });
  const cookieHeaders = getSetCookieHeaders(source);
  for (const cookieHeader of cookieHeaders) {
    headers.append("set-cookie", cookieHeader);
  }
  return headers;
}

function getSetCookieHeaders(headers: Headers) {
  const source = headers as Headers & {
    getSetCookie?: () => string[];
  };
  if (typeof source.getSetCookie === "function") {
    return source.getSetCookie();
  }
  const fallback = headers.get("set-cookie");
  if (!fallback) {
    return [];
  }
  return [fallback];
}

async function handleProxy(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  const target = buildTargetURL(req, path);
  const response = await fetch(target, {
    method: req.method,
    headers: copyRequestHeaders(req),
    body: await readRequestBody(req),
    redirect: "manual",
    cache: "no-store",
  });
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: copyResponseHeaders(response.headers),
  });
}

export const dynamic = "force-dynamic";

export async function GET(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}

export async function POST(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}

export async function PUT(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}

export async function PATCH(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}

export async function DELETE(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}

export async function OPTIONS(req: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  return handleProxy(req, context);
}
