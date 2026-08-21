import { gql, type Team } from './api';

const TEAM_FIELDS = `id name createdAt updatedAt`;

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
