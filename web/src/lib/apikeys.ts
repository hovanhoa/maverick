import { gql, type ApiKey, type ApiKeySecret } from './api';

const API_KEY_FIELDS = `id accountId prefix createdAt revokedAt`;

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
