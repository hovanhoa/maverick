export type Role = 'OWNER' | 'ADMIN' | 'MEMBER';

export interface Account {
  id: string;
  email: string;
  username: string;
  teamId: string | null;
  role: Role;
  createdAt: string;
  updatedAt: string;
}

export interface Team {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  modelAllowlist: string[];
  monthlyTokenBudget: number | null;
}

export interface UsageSummary {
  requestCount: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  costUsd: number;
}

export interface ApiKey {
  id: string;
  accountId: string;
  prefix: string;
  createdAt: string;
  revokedAt: string | null;
}

export interface ApiKeySecret {
  apiKey: ApiKey;
  key: string;
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
