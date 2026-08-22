import * as React from 'react';
import { useNavigate } from 'react-router-dom';
import { PlusIcon, KeyIcon, PencilIcon, TrashIcon } from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { RoleBadge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { Modal } from '../components/ui/Modal';
import { Field, TextInput, Select } from '../components/ui/Field';
import { listAccounts, createAccount, updateAccount, deleteAccount } from '../lib/accounts';
import { listTeams } from '../lib/teams';
import { useAuth } from '../lib/auth';
import type { Account, Role, Team } from '../lib/api';

export function AccountsPage() {
  const navigate = useNavigate();
  const { account: me, isOwnerOrAdmin } = useAuth();
  const [accounts, setAccounts] = React.useState<Account[]>([]);
  const [teams, setTeams] = React.useState<Team[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Account | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [accs, tms] = await Promise.all([listAccounts(), listTeams()]);
      setAccounts(accs);
      setTeams(tms);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const teamName = (teamId: string | null) => teams.find((t) => t.id === teamId)?.name ?? '—';

  const handleDelete = async (account: Account) => {
    if (!confirm(`Delete account ${account.email}? This cannot be undone.`)) return;
    setError(null);
    try {
      await deleteAccount(account.id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">Accounts</h1>
          <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">Your team's accounts, and your own if you have no team yet.</p>
        </div>
        <Button variant="primary" onClick={() => setCreateOpen(true)}>
          <PlusIcon className="h-4 w-4" />
          New account
        </Button>
      </div>

      {error && <ErrorAlert message={error} />}

      <Card>
        <CardHeader title={`${accounts.length} account${accounts.length === 1 ? '' : 's'}`} />
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-xs uppercase text-neutral-500 dark:text-neutral-400">
              <tr>
                <th className="px-5 py-2.5 font-medium">Email</th>
                <th className="px-5 py-2.5 font-medium">Username</th>
                <th className="px-5 py-2.5 font-medium">Role</th>
                <th className="px-5 py-2.5 font-medium">Team</th>
                <th className="px-5 py-2.5 font-medium">Created</th>
                <th className="px-5 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-950/5 dark:divide-white/10">
              {accounts.map((account) => (
                <tr key={account.id} className="hover:bg-neutral-950/[0.02] dark:hover:bg-white/[0.02]">
                  <td className="px-5 py-3 text-neutral-900 dark:text-white">{account.email}</td>
                  <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{account.username}</td>
                  <td className="px-5 py-3">
                    <RoleBadge role={account.role} />
                  </td>
                  <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{teamName(account.teamId)}</td>
                  <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{new Date(account.createdAt).toLocaleDateString()}</td>
                  <td className="px-5 py-3 text-right">
                    <div className="flex justify-end gap-1">
                      {(isOwnerOrAdmin || account.id === me?.id) && (
                        <Button
                          variant="ghost"
                          className="px-2"
                          title="Manage API keys"
                          onClick={() => navigate(`/api-keys?accountId=${account.id}`)}
                        >
                          <KeyIcon className="h-4 w-4" />
                        </Button>
                      )}
                      <Button variant="ghost" className="px-2" title="Edit" onClick={() => setEditing(account)}>
                        <PencilIcon className="h-4 w-4" />
                      </Button>
                      {isOwnerOrAdmin && (
                        <Button variant="ghost" className="px-2 text-red-600 dark:text-red-400" title="Delete" onClick={() => handleDelete(account)}>
                          <TrashIcon className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && accounts.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-sm text-neutral-400">
                    No accounts yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <CreateAccountModal
        open={createOpen}
        teams={teams}
        canSetRole={isOwnerOrAdmin}
        onClose={() => setCreateOpen(false)}
        onCreated={async () => {
          setCreateOpen(false);
          await load();
        }}
      />
      <EditAccountModal
        account={editing}
        teams={teams}
        canSetRole={isOwnerOrAdmin}
        onClose={() => setEditing(null)}
        onSaved={async () => {
          setEditing(null);
          await load();
        }}
      />
    </div>
  );
}

function CreateAccountModal({
  open,
  teams,
  canSetRole,
  onClose,
  onCreated
}: {
  open: boolean;
  teams: Team[];
  canSetRole: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [email, setEmail] = React.useState('');
  const [username, setUsername] = React.useState('');
  const [teamId, setTeamId] = React.useState('');
  const [role, setRole] = React.useState<Role>('MEMBER');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setEmail('');
      setUsername('');
      setTeamId('');
      setRole('MEMBER');
      setError(null);
    }
  }, [open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await createAccount({ email, username, teamId: teamId || undefined, role });
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="New account">
      <form onSubmit={handleSubmit} className="space-y-3">
        <Field label="Email">
          <TextInput required type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label="Username">
          <TextInput required value={username} onChange={(e) => setUsername(e.target.value)} />
        </Field>
        <Field label="Team (optional)">
          <Select value={teamId} onChange={(e) => setTeamId(e.target.value)}>
            <option value="">No team</option>
            {teams.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        </Field>
        {canSetRole ? (
          <Field label="Role">
            <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              <option value="MEMBER">MEMBER</option>
              <option value="ADMIN">ADMIN</option>
              <option value="OWNER">OWNER</option>
            </Select>
          </Field>
        ) : (
          <p className="text-xs text-neutral-400">Role locked to MEMBER - requires OWNER or ADMIN to set otherwise.</p>
        )}
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Creating…' : 'Create account'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function EditAccountModal({
  account,
  teams,
  canSetRole,
  onClose,
  onSaved
}: {
  account: Account | null;
  teams: Team[];
  canSetRole: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [email, setEmail] = React.useState('');
  const [username, setUsername] = React.useState('');
  const [teamId, setTeamId] = React.useState('');
  const [role, setRole] = React.useState<Role>('MEMBER');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (account) {
      setEmail(account.email);
      setUsername(account.username);
      setTeamId(account.teamId ?? '');
      setRole(account.role);
      setError(null);
    }
  }, [account]);

  if (!account) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await updateAccount({
        id: account.id,
        email: email !== account.email ? email : undefined,
        username: username !== account.username ? username : undefined,
        teamId: teamId && teamId !== account.teamId ? teamId : undefined,
        clearTeamId: !teamId && account.teamId ? true : undefined,
        role: role !== account.role ? role : undefined
      });
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Edit ${account.username}`}>
      <form onSubmit={handleSubmit} className="space-y-3">
        <Field label="Email">
          <TextInput required type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label="Username">
          <TextInput required value={username} onChange={(e) => setUsername(e.target.value)} />
        </Field>
        <Field label="Team">
          <Select value={teamId} onChange={(e) => setTeamId(e.target.value)}>
            <option value="">No team</option>
            {teams.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Role">
          {canSetRole ? (
            <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              <option value="MEMBER">MEMBER</option>
              <option value="ADMIN">ADMIN</option>
              <option value="OWNER">OWNER</option>
            </Select>
          ) : (
            <div className="flex items-center justify-between rounded-md bg-neutral-50 px-3 py-2 text-sm text-neutral-500 ring-1 ring-inset ring-neutral-200 dark:bg-white/5 dark:text-neutral-400 dark:ring-white/10">
              {role}
              <span className="text-xs text-neutral-400">Locked</span>
            </div>
          )}
        </Field>
        {!canSetRole && (
          <p className="text-xs text-neutral-400">Changing role or deleting an account requires OWNER or ADMIN.</p>
        )}
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save changes'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
