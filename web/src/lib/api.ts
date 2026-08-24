export type Role = 'OWNER' | 'ADMIN' | 'MEMBER';

export interface Account {
  id: string;
  email: string;
  username: string;
  name: string | null;
  teamId: string | null;
  role: Role;
  createdAt: string;
  updatedAt: string;
}

/** URL for an account's profile picture. Always resolves - a missing avatar 404s, so render with an onError fallback. */
export function avatarUrl(accountId: string): string {
  return `${API_URL}/accounts/${accountId}/avatar`;
}

/** Uploads (or replaces) the caller's own avatar. Self-service only - the server rejects anyone else's id. */
export async function uploadAvatar(accountId: string, file: File): Promise<void> {
  const res = await fetch(avatarUrl(accountId), {
    method: 'POST',
    headers: {
      'Content-Type': file.type,
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {})
    },
    body: file
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `upload failed with status ${res.status}`);
  }
}

/** Removes the caller's own avatar, if any. */
export async function deleteAvatar(accountId: string): Promise<void> {
  const res = await fetch(avatarUrl(accountId), {
    method: 'DELETE',
    headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {}
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `delete failed with status ${res.status}`);
  }
}

/** Returned once, when an account's password is set (at creation) or reset - never retrievable again. */
export interface AccountSecret {
  account: Account;
  password: string;
}

export interface TeamPolicy {
  blockedPatterns: string[];
  denyOnSensitiveData: boolean;
}

export interface Team {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  modelAllowlist: string[];
  monthlyTokenBudget: number | null;
  policy: TeamPolicy;
}

export interface UsageSummary {
  requestCount: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  costUsd: number;
}

export interface AccountUsage extends UsageSummary {
  /** Null groups together usage from any account(s) since deleted. */
  accountId: string | null;
}

export interface ModelUsage extends UsageSummary {
  provider: string;
  model: string;
}

export interface TeamUsage extends UsageSummary {
  teamId: string;
  name: string;
}

export interface DailyUsage {
  date: string;
  requestCount: number;
  totalTokens: number;
  costUsd: number;
}

export interface ApiKey {
  id: string;
  accountId: string;
  prefix: string;
  createdAt: string;
  revokedAt: string | null;
  lastUsedAt: string | null;
}

export interface ApiKeySecret {
  apiKey: ApiKey;
  key: string;
}

export type RequestLogStatus = 'SUCCESS' | 'ERROR';

export interface RequestLog {
  id: string;
  requestId: string;
  /** Null if the account that made this call has since been deleted. */
  accountId: string | null;
  teamId: string | null;
  provider: string | null;
  model: string | null;
  requestedModel: string;
  status: RequestLogStatus;
  errorKind: string | null;
  errorMessage: string | null;
  stream: boolean;
  requestBody: string;
  responseBody: string | null;
  promptTokens: number | null;
  completionTokens: number | null;
  totalTokens: number | null;
  costUsd: number | null;
  latencyMs: number;
  createdAt: string;
}

export interface RequestLogConnection {
  items: RequestLog[];
  totalCount: number;
  hasNextPage: boolean;
}

export type ChatRole = 'system' | 'user' | 'assistant';

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface ChatCompletionRequest {
  model: string;
  messages: ChatMessage[];
  temperature?: number;
  max_tokens?: number;
}

export interface ChatUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface ChatCompletionResponse {
  id: string;
  model: string;
  choices: { index: number; message: ChatMessage; finish_reason: string }[];
  usage: ChatUsage;
}

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export class GraphQLError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'GraphQLError';
  }
}

let apiKey: string | null = localStorage.getItem('llmgateway.apiKey');

export function getApiKey(): string | null {
  return apiKey;
}

export function setApiKey(key: string | null): void {
  apiKey = key;
  if (key) {
    localStorage.setItem('llmgateway.apiKey', key);
  } else {
    localStorage.removeItem('llmgateway.apiKey');
  }
}

/** Sends a single GraphQL request and returns its `data`, throwing a GraphQLError if the response carries any errors. */
export async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const res = await fetch(`${API_URL}/graphql/query`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {})
    },
    body: JSON.stringify({ query, variables })
  });

  if (res.status === 401) {
    throw new GraphQLError('authentication required - the API key is missing, invalid, or revoked');
  }

  const body = (await res.json()) as { data?: T; errors?: { message: string }[] };
  if (body.errors?.length) {
    throw new GraphQLError(body.errors.map((e) => e.message).join('; '));
  }
  if (!body.data) {
    throw new GraphQLError('empty response from server');
  }
  return body.data;
}

export interface LoginResult {
  key: string;
  account: Account;
}

/**
 * Logs in with a username and password, returning a fresh API key - the
 * same kind issued from the API Keys page, just obtained without already
 * having one. Deliberately hits the plain /login REST endpoint rather than
 * GraphQL: every GraphQL operation requires an existing API key, which is
 * exactly what login doesn't have yet.
 */
export async function login(username: string, password: string): Promise<LoginResult> {
  const res = await fetch(`${API_URL}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });

  const body = await res.json();
  if (!res.ok) {
    throw new Error(body?.Error?.Message ?? body?.error?.message ?? 'invalid username or password');
  }
  return body as LoginResult;
}

/**
 * Sends a single non-streaming chat completion using an arbitrary API key,
 * bypassing the session's stored key. Used by the Playground page to let
 * someone test a key against the live gateway without leaving the browser.
 */
export async function testChatCompletion(key: string, req: ChatCompletionRequest): Promise<ChatCompletionResponse> {
  const res = await fetch(`${API_URL}/v1/chat/completions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${key}`
    },
    body: JSON.stringify({ ...req, stream: false })
  });

  const body = await res.json();
  if (!res.ok) {
    const message = body?.error?.message ?? `request failed with status ${res.status}`;
    throw new Error(message);
  }
  return body as ChatCompletionResponse;
}
