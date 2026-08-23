import { gql, type RequestLogConnection } from './api';

const REQUEST_LOG_FIELDS = `
  id requestId accountId teamId provider model requestedModel status
  errorKind errorMessage stream requestBody responseBody promptTokens
  completionTokens totalTokens costUsd latencyMs createdAt
`;

const CONNECTION_FIELDS = `items { ${REQUEST_LOG_FIELDS} } totalCount hasNextPage`;

export async function getTeamRequestLogs(teamId: string, limit = 20, offset = 0): Promise<RequestLogConnection> {
  const data = await gql<{ teamRequestLogs: RequestLogConnection }>(
    `query($teamId: ID!, $limit: Int, $offset: Int) { teamRequestLogs(teamId: $teamId, limit: $limit, offset: $offset) { ${CONNECTION_FIELDS} } }`,
    { teamId, limit, offset }
  );
  return data.teamRequestLogs;
}

export async function getMyRequestLogs(limit = 20, offset = 0): Promise<RequestLogConnection> {
  const data = await gql<{ myRequestLogs: RequestLogConnection }>(
    `query($limit: Int, $offset: Int) { myRequestLogs(limit: $limit, offset: $offset) { ${CONNECTION_FIELDS} } }`,
    { limit, offset }
  );
  return data.myRequestLogs;
}

export async function getGlobalRequestLogs(limit = 20, offset = 0): Promise<RequestLogConnection> {
  const data = await gql<{ globalRequestLogs: RequestLogConnection }>(
    `query($limit: Int, $offset: Int) { globalRequestLogs(limit: $limit, offset: $offset) { ${CONNECTION_FIELDS} } }`,
    { limit, offset }
  );
  return data.globalRequestLogs;
}
