import { gql, type Account, type AccountSecret, type Role } from './api';

const ACCOUNT_FIELDS = `id email username name teamId role createdAt updatedAt`;

export async function listAccounts(teamId?: string): Promise<Account[]> {
  const data = await gql<{ accounts: { items: Account[] } }>(
    `query($teamId: ID) { accounts(teamId: $teamId, limit: 100) { items { ${ACCOUNT_FIELDS} } } }`,
    { teamId }
  );
  return data.accounts.items;
}

/** Cheap member count for a team - fetches just the connection's totalCount, not the accounts themselves. */
export async function getTeamMemberCount(teamId: string): Promise<number> {
  const data = await gql<{ accounts: { totalCount: number } }>(
    `query($teamId: ID!) { accounts(teamId: $teamId, limit: 1) { totalCount } }`,
    { teamId }
  );
  return data.accounts.totalCount;
}

/** The account backing the current API key. Used to know what the signed-in caller is allowed to do. */
export async function getMe(): Promise<Account> {
  const data = await gql<{ me: Account }>(`query { me { ${ACCOUNT_FIELDS} } }`);
  return data.me;
}

export async function createAccount(input: {
  email: string;
  username: string;
  teamId?: string;
  role?: Role;
}): Promise<AccountSecret> {
  const data = await gql<{ createAccount: AccountSecret }>(
    `mutation($email: String!, $username: String!, $teamId: ID, $role: Role) {
      createAccount(email: $email, username: $username, teamId: $teamId, role: $role) {
        account { ${ACCOUNT_FIELDS} }
        password
      }
    }`,
    input
  );
  return data.createAccount;
}

/** Generates a new random password for an account, returned once. Requires OWNER/ADMIN. */
export async function resetAccountPassword(id: string): Promise<AccountSecret> {
  const data = await gql<{ resetAccountPassword: AccountSecret }>(
    `mutation($id: ID!) {
      resetAccountPassword(id: $id) {
        account { ${ACCOUNT_FIELDS} }
        password
      }
    }`,
    { id }
  );
  return data.resetAccountPassword;
}

export async function updateAccount(input: {
  id: string;
  email?: string;
  username?: string;
  name?: string;
  teamId?: string;
  clearTeamId?: boolean;
  role?: Role;
}): Promise<Account> {
  const data = await gql<{ updateAccount: Account }>(
    `mutation($id: ID!, $email: String, $username: String, $name: String, $teamId: ID, $clearTeamId: Boolean, $role: Role) {
      updateAccount(id: $id, email: $email, username: $username, name: $name, teamId: $teamId, clearTeamId: $clearTeamId, role: $role) { ${ACCOUNT_FIELDS} }
    }`,
    input
  );
  return data.updateAccount;
}

export async function deleteAccount(id: string): Promise<boolean> {
  const data = await gql<{ deleteAccount: boolean }>(`mutation($id: ID!) { deleteAccount(id: $id) }`, { id });
  return data.deleteAccount;
}
