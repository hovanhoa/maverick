import { gql, type Account, type Role } from './api';

const ACCOUNT_FIELDS = `id email username teamId role createdAt updatedAt`;

export async function listAccounts(): Promise<Account[]> {
  const data = await gql<{ accounts: { items: Account[] } }>(
    `query { accounts(limit: 100) { items { ${ACCOUNT_FIELDS} } } }`
  );
  return data.accounts.items;
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
}): Promise<Account> {
  const data = await gql<{ createAccount: Account }>(
    `mutation($email: String!, $username: String!, $teamId: ID, $role: Role) {
      createAccount(email: $email, username: $username, teamId: $teamId, role: $role) { ${ACCOUNT_FIELDS} }
    }`,
    input
  );
  return data.createAccount;
}

export async function updateAccount(input: {
  id: string;
  email?: string;
  username?: string;
  teamId?: string;
  clearTeamId?: boolean;
  role?: Role;
}): Promise<Account> {
  const data = await gql<{ updateAccount: Account }>(
    `mutation($id: ID!, $email: String, $username: String, $teamId: ID, $clearTeamId: Boolean, $role: Role) {
      updateAccount(id: $id, email: $email, username: $username, teamId: $teamId, clearTeamId: $clearTeamId, role: $role) { ${ACCOUNT_FIELDS} }
    }`,
    input
  );
  return data.updateAccount;
}

export async function deleteAccount(id: string): Promise<boolean> {
  const data = await gql<{ deleteAccount: boolean }>(`mutation($id: ID!) { deleteAccount(id: $id) }`, { id });
  return data.deleteAccount;
}
