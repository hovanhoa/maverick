import * as React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { PlusIcon, KeyIcon, PencilIcon, TrashIcon, LockClosedIcon, ClipboardDocumentIcon } from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { RoleBadge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { Modal } from '../components/ui/Modal';
import { Field, TextInput, Select } from '../components/ui/Field';
import { listAccounts, createAccount, updateAccount, deleteAccount, resetAccountPassword } from '../lib/accounts';
import { listTeams } from '../lib/teams';
import { useAuth } from '../lib/auth';
import { avatarUrl, uploadAvatar, deleteAvatar } from '../lib/api';
import type { Account, AccountSecret, Role, Team } from '../lib/api';

/**
 * Small circular avatar with an initials fallback for accounts with no
 * picture set (or a broken/missing image). `version` busts the browser's
 * image cache after an upload/delete - it doesn't change on its own just
 * because some other field on the account was edited.
 */
function Avatar({ account, version = 0, size = 8 }: { account: Account; version?: number; size?: number }) {
  const [broken, setBroken] = React.useState(false);
  const initials = (account.name || account.username).slice(0, 2).toUpperCase();
  const dimension = `${size * 0.25}rem`;

  React.useEffect(() => setBroken(false), [version]);

  if (broken) {
    return (
      <div
        style={{ width: dimension, height: dimension }}
        className="flex shrink-0 items-center justify-center rounded-full bg-neutral-200 text-xs font-medium text-neutral-600 dark:bg-white/10 dark:text-neutral-300"
      >
        {initials}
      </div>
    );
  }

  return (
    <img
      src={`${avatarUrl(account.id)}?v=${version}`}
      alt=""
      style={{ width: dimension, height: dimension }}
      className="shrink-0 rounded-full bg-neutral-200 object-cover dark:bg-white/10"
      onError={() => setBroken(true)}
    />
  );
}

export function AccountsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { account: me, isOwnerOrAdmin, refreshAccount } = useAuth();
  const [accounts, setAccounts] = React.useState<Account[]>([]);
  const [teams, setTeams] = React.useState<Team[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<Account | null>(null);
  const [secret, setSecret] = React.useState<AccountSecret | null>(null);
  const [resetting, setResetting] = React.useState<string | null>(null);
  const [avatarVersion, setAvatarVersion] = React.useState(0);

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

  // Clicking the sidebar profile block navigates here with editSelf in
  // router state, so it opens straight into "edit my own account" instead
  // of landing on the plain list. Cleared via replace so a later refresh or
  // back-navigation doesn't reopen it.
  React.useEffect(() => {
    const state = location.state as { editSelf?: boolean } | null;
    if (state?.editSelf && me) {
      setEditing(me);
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location, me, navigate]);

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

  const handleResetPassword = async (account: Account) => {
    if (!confirm(`Reset the password for ${account.email}? Their current password will stop working immediately.`)) return;
    setError(null);
    setResetting(account.id);
    try {
      setSecret(await resetAccountPassword(account.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setResetting(null);
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
                <th className="px-5 py-2.5 font-medium">Account</th>
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
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2.5">
                      <Avatar account={account} version={account.id === me?.id ? avatarVersion : 0} />
                      <div className="min-w-0">
                        <p className="truncate font-medium text-neutral-900 dark:text-white">{account.name || account.username}</p>
                        <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{account.email}</p>
                      </div>
                    </div>
                  </td>
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
                        <Button
                          variant="ghost"
                          className="px-2"
                          title="Reset password"
                          disabled={resetting === account.id}
                          onClick={() => handleResetPassword(account)}
                        >
                          <LockClosedIcon className="h-4 w-4" />
                        </Button>
                      )}
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
        onCreated={async (created) => {
          setCreateOpen(false);
          setSecret(created);
          await load();
        }}
      />
      <EditAccountModal
        account={editing}
        teams={teams}
        canSetRole={isOwnerOrAdmin}
        isSelf={editing !== null && editing.id === me?.id}
        avatarVersion={avatarVersion}
        onAvatarChanged={() => {
          setAvatarVersion((v) => v + 1);
          refreshAccount();
        }}
        onClose={() => setEditing(null)}
        onSaved={async () => {
          const wasSelf = editing?.id === me?.id;
          setEditing(null);
          await load();
          if (wasSelf) refreshAccount();
        }}
      />
      <PasswordSecretModal secret={secret} onClose={() => setSecret(null)} />
    </div>
  );
}

/** Shown once after an account's password is created or reset - it can't be retrieved again after this. */
function PasswordSecretModal({ secret, onClose }: { secret: AccountSecret | null; onClose: () => void }) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = () => {
    if (!secret) return;
    navigator.clipboard.writeText(secret.password);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Modal open={secret !== null} onClose={onClose} title="Password set">
      {secret && (
        <div className="space-y-3">
          <p className="text-sm text-neutral-500 dark:text-neutral-400">
            Store this now for <span className="font-medium text-neutral-700 dark:text-neutral-300">{secret.account.username}</span> - it
            won't be shown again. Sign in at /login with this username and password.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded-md bg-neutral-50 px-3 py-2 text-sm text-neutral-900 ring-1 ring-inset ring-neutral-200 dark:bg-neutral-950 dark:text-white dark:ring-white/10">
              {secret.password}
            </code>
            <Button onClick={handleCopy} title="Copy to clipboard">
              <ClipboardDocumentIcon className="h-4 w-4" />
              {copied ? 'Copied' : 'Copy'}
            </Button>
          </div>
          <div className="flex justify-end pt-2">
            <Button variant="primary" onClick={onClose}>
              Done
            </Button>
          </div>
        </div>
      )}
    </Modal>
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
  onCreated: (secret: AccountSecret) => void;
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
      const created = await createAccount({ email, username, teamId: teamId || undefined, role });
      onCreated(created);
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
  isSelf,
  avatarVersion,
  onAvatarChanged,
  onClose,
  onSaved
}: {
  account: Account | null;
  teams: Team[];
  canSetRole: boolean;
  isSelf: boolean;
  avatarVersion: number;
  onAvatarChanged: () => void;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [email, setEmail] = React.useState('');
  const [username, setUsername] = React.useState('');
  const [name, setName] = React.useState('');
  const [teamId, setTeamId] = React.useState('');
  const [role, setRole] = React.useState<Role>('MEMBER');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  const [avatarError, setAvatarError] = React.useState<string | null>(null);
  const [avatarBusy, setAvatarBusy] = React.useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    if (account) {
      setEmail(account.email);
      setUsername(account.username);
      setName(account.name ?? '');
      setTeamId(account.teamId ?? '');
      setRole(account.role);
      setError(null);
      setAvatarError(null);
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
        name: name !== (account.name ?? '') ? name : undefined,
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

  const handleAvatarPick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setAvatarBusy(true);
    setAvatarError(null);
    try {
      await uploadAvatar(account.id, file);
      onAvatarChanged();
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : String(err));
    } finally {
      setAvatarBusy(false);
    }
  };

  const handleAvatarRemove = async () => {
    setAvatarBusy(true);
    setAvatarError(null);
    try {
      await deleteAvatar(account.id);
      onAvatarChanged();
    } catch (err) {
      setAvatarError(err instanceof Error ? err.message : String(err));
    } finally {
      setAvatarBusy(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Edit ${account.username}`}>
      {isSelf && (
        <div className="mb-4 flex items-center gap-3 border-b border-neutral-950/5 pb-4 dark:border-white/10">
          <Avatar account={account} version={avatarVersion} size={14} />
          <div className="space-y-1.5">
            <div className="flex gap-2">
              <Button type="button" disabled={avatarBusy} onClick={() => fileInputRef.current?.click()}>
                {avatarBusy ? 'Working…' : 'Change picture'}
              </Button>
              <Button type="button" variant="ghost" disabled={avatarBusy} onClick={handleAvatarRemove}>
                Remove
              </Button>
            </div>
            <input ref={fileInputRef} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onChange={handleAvatarPick} />
            {avatarError && <p className="text-xs text-red-600 dark:text-red-400">{avatarError}</p>}
          </div>
        </div>
      )}
      <form onSubmit={handleSubmit} className="space-y-3">
        <Field label="Email">
          <TextInput required type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label="Username">
          <TextInput required value={username} onChange={(e) => setUsername(e.target.value)} />
        </Field>
        <Field label="Display name (optional)">
          <TextInput value={name} onChange={(e) => setName(e.target.value)} placeholder={account.username} />
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
