type ServiceName = 'core' | 'saas';

const coreBaseUrl = import.meta.env.VITE_NEXUS_CORE_URL || 'http://localhost:8080';
const saasBaseUrl = import.meta.env.VITE_NEXUS_SAAS_URL || 'http://localhost:8082';

const devFallbackApiKey = (import.meta.env.VITE_NEXUS_API_KEY || '').trim();
const devFallbackScopes =
  (import.meta.env.VITE_NEXUS_SCOPES || '').trim() ||
  'tools:read,tools:write,policy:read,policy:write,egress:read,egress:write,audit:read,gateway:run,gateway:simulate,admin:secrets,admin:console:read,admin:console:write';

const allowDevAPIKeyFallback = import.meta.env.DEV || import.meta.env.VITE_ALLOW_API_KEY_FALLBACK === 'true';

type TokenGetter = () => Promise<string | null>;

let getTokenFn: TokenGetter | null = null;

export function setTokenGetter(getter: TokenGetter | null) {
  getTokenFn = getter;
}

function serviceBaseURL(service: ServiceName): string {
  return service === 'core' ? coreBaseUrl : saasBaseUrl;
}

function normalizeHeaders(input?: HeadersInit): Headers {
  if (!input) return new Headers();
  return new Headers(input);
}

async function authHeaders(): Promise<Headers> {
  const headers = new Headers();
  const token = getTokenFn ? await getTokenFn() : null;
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
    return headers;
  }
  if (allowDevAPIKeyFallback && devFallbackApiKey) {
    headers.set('X-NEXUS-CORE-KEY', devFallbackApiKey);
    headers.set('X-NEXUS-SCOPES', devFallbackScopes);
    headers.set('X-NEXUS-ACTOR', 'tower/ui');
  }
  return headers;
}

function hasBody(init?: RequestInit): boolean {
  if (!init) return false;
  return init.body != null;
}

function isJSONContentType(headers: Headers): boolean {
  const v = headers.get('Content-Type');
  return v != null && v.toLowerCase().includes('application/json');
}

async function parseError(res: Response): Promise<string> {
  const text = await res.text();
  if (!text) return `API ${res.status}`;
  try {
    const obj = JSON.parse(text) as { error?: { message?: string } };
    const msg = obj?.error?.message;
    if (msg) return `API ${res.status}: ${msg}`;
  } catch {
    // no-op
  }
  return `API ${res.status}: ${text}`;
}

export async function requestJSON<T>(service: ServiceName, path: string, init?: RequestInit): Promise<T> {
  const headers = normalizeHeaders(init?.headers);
  const auth = await authHeaders();
  auth.forEach((value, key) => {
    if (!headers.has(key)) {
      headers.set(key, value);
    }
  });
  if (hasBody(init) && !isJSONContentType(headers)) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(`${serviceBaseURL(service)}${path}`, { ...init, headers });
  if (!res.ok) {
    throw new Error(await parseError(res));
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

