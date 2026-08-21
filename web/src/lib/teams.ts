import { gql, type Team } from './api';

const TEAM_FIELDS = `id name createdAt updatedAt modelAllowlist`;

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
