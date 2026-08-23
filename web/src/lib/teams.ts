import { gql, type Team, type UsageSummary } from './api';

const TEAM_FIELDS = `id name createdAt updatedAt modelAllowlist monthlyTokenBudget policy { blockedPatterns denyOnSensitiveData }`;

export async function listTeams(): Promise<Team[]> {
  const data = await gql<{ teams: { items: Team[] } }>(`query { teams(limit: 100) { items { ${TEAM_FIELDS} } } }`);
  return data.teams.items;
}

export async function createTeam(name: string): Promise<Team> {
  const data = await gql<{ createTeam: Team }>(
    `mutation($name: String!) { createTeam(name: $name) { ${TEAM_FIELDS} } }`,
    { name }
  );
  return data.createTeam;
}

export async function updateTeam(id: string, name: string): Promise<Team> {
  const data = await gql<{ updateTeam: Team }>(
    `mutation($id: ID!, $name: String!) { updateTeam(id: $id, name: $name) { ${TEAM_FIELDS} } }`,
    { id, name }
  );
  return data.updateTeam;
}

export async function deleteTeam(id: string): Promise<boolean> {
  const data = await gql<{ deleteTeam: boolean }>(`mutation($id: ID!) { deleteTeam(id: $id) }`, { id });
  return data.deleteTeam;
}

export async function updateTeamModelAllowlist(teamId: string, allowlist: string[]): Promise<Team> {
  const data = await gql<{ updateTeamModelAllowlist: Team }>(
    `mutation($teamId: ID!, $allowlist: [String!]!) { updateTeamModelAllowlist(teamId: $teamId, allowlist: $allowlist) { ${TEAM_FIELDS} } }`,
    { teamId, allowlist }
  );
  return data.updateTeamModelAllowlist;
}

export async function isModelAllowed(teamId: string, provider: string, model: string): Promise<boolean> {
  const data = await gql<{ isModelAllowed: boolean }>(
    `query($teamId: ID!, $provider: String!, $model: String!) { isModelAllowed(teamId: $teamId, provider: $provider, model: $model) }`,
    { teamId, provider, model }
  );
  return data.isModelAllowed;
}

/** Sets the team's monthly token budget, or clears it (unlimited) when budget is null. */
export async function updateTeamQuota(teamId: string, budget: number | null): Promise<Team> {
  const data = await gql<{ updateTeamQuota: Team }>(
    `mutation($teamId: ID!, $budget: Int, $clear: Boolean) {
      updateTeamQuota(teamId: $teamId, monthlyTokenBudget: $budget, clearMonthlyTokenBudget: $clear) { ${TEAM_FIELDS} }
    }`,
    { teamId, budget, clear: budget === null }
  );
  return data.updateTeamQuota;
}

/** Replaces a team's content-policy overrides wholesale. */
export async function updateTeamPolicy(teamId: string, blockedPatterns: string[], denyOnSensitiveData: boolean): Promise<Team> {
  const data = await gql<{ updateTeamPolicy: Team }>(
    `mutation($teamId: ID!, $blockedPatterns: [String!]!, $denyOnSensitiveData: Boolean!) {
      updateTeamPolicy(teamId: $teamId, blockedPatterns: $blockedPatterns, denyOnSensitiveData: $denyOnSensitiveData) { ${TEAM_FIELDS} }
    }`,
    { teamId, blockedPatterns, denyOnSensitiveData }
  );
  return data.updateTeamPolicy;
}

export async function getTeamUsage(teamId: string, since?: string): Promise<UsageSummary> {
  const data = await gql<{ teamUsage: UsageSummary }>(
    `query($teamId: ID!, $since: Time) { teamUsage(teamId: $teamId, since: $since) { requestCount promptTokens completionTokens totalTokens costUsd } }`,
    { teamId, since }
  );
  return data.teamUsage;
}
