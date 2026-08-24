import { gql, type ApiKey, type ApiKeySecret } from './api';

const API_KEY_FIELDS = `id accountId prefix createdAt revokedAt lastUsedAt monthlyTokenBudget`;

export async function listApiKeys(accountId: string): Promise<ApiKey[]> {
  const data = await gql<{ apiKeys: ApiKey[] }>(
    `query($accountId: ID!) { apiKeys(accountId: $accountId) { ${API_KEY_FIELDS} } }`,
    { accountId }
  );
  return data.apiKeys;
}

export async function createApiKey(accountId: string): Promise<ApiKeySecret> {
  const data = await gql<{ createApiKey: ApiKeySecret }>(
    `mutation($accountId: ID!) { createApiKey(accountId: $accountId) { key apiKey { ${API_KEY_FIELDS} } } }`,
    { accountId }
  );
  return data.createApiKey;
}

export async function revokeApiKey(id: string): Promise<boolean> {
  const data = await gql<{ revokeApiKey: boolean }>(`mutation($id: ID!) { revokeApiKey(id: $id) }`, { id });
  return data.revokeApiKey;
}

/** Sets or clears an API key's monthly token budget. Pass null to clear it (unlimited). */
export async function updateApiKeyQuota(id: string, budget: number | null): Promise<ApiKey> {
  const data = await gql<{ updateApiKeyQuota: ApiKey }>(
    `mutation($id: ID!, $budget: Int, $clear: Boolean) {
      updateApiKeyQuota(id: $id, monthlyTokenBudget: $budget, clearMonthlyTokenBudget: $clear) { ${API_KEY_FIELDS} }
    }`,
    { id, budget, clear: budget === null }
  );
  return data.updateApiKeyQuota;
}
