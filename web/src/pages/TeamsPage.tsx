import * as React from 'react';
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  CheckIcon,
  XMarkIcon,
  AdjustmentsHorizontalIcon,
  ChartBarIcon,
  ShieldExclamationIcon,
  UserGroupIcon
} from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge, RoleBadge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { Modal } from '../components/ui/Modal';
import { Field, TextInput, Textarea } from '../components/ui/Field';
import {
  listTeams,
  createTeam,
  updateTeam,
  deleteTeam,
  updateTeamModelAllowlist,
  isModelAllowed,
  updateTeamQuota,
  updateTeamPolicy,
  getTeamUsage
} from '../lib/teams';
import { listAccounts, getTeamMemberCount, createAccount, updateAccount } from '../lib/accounts';
import { useAuth } from '../lib/auth';
import type { Account, Team, UsageSummary } from '../lib/api';

export function TeamsPage() {
  const { isOwnerOrAdmin } = useAuth();
  const [teams, setTeams] = React.useState<Team[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [renamingId, setRenamingId] = React.useState<string | null>(null);
  const [renameValue, setRenameValue] = React.useState('');
  const [editingAllowlist, setEditingAllowlist] = React.useState<Team | null>(null);
  const [editingQuota, setEditingQuota] = React.useState<Team | null>(null);
  const [editingPolicy, setEditingPolicy] = React.useState<Team | null>(null);
  const [viewingMembers, setViewingMembers] = React.useState<Team | null>(null);
  const [memberCounts, setMemberCounts] = React.useState<Record<string, number>>({});

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listTeams();
      setTeams(list);
      const counts = await Promise.all(list.map((t) => getTeamMemberCount(t.id)));
      setMemberCounts(Object.fromEntries(list.map((t, i) => [t.id, counts[i]])));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const startRename = (team: Team) => {
    setRenamingId(team.id);
    setRenameValue(team.name);
  };

  const commitRename = async (id: string) => {
    setError(null);
    try {
      await updateTeam(id, renameValue);
      setRenamingId(null);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleDelete = async (team: Team) => {
    if (!confirm(`Delete team "${team.name}"? This cannot be undone.`)) return;
    setError(null);
    try {
      await deleteTeam(team.id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">Teams</h1>
          <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">Creating a team makes you its OWNER.</p>
        </div>
        {isOwnerOrAdmin && (
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <PlusIcon className="h-4 w-4" />
            New team
          </Button>
        )}
      </div>

      {!isOwnerOrAdmin && <ErrorAlert message="Team management requires OWNER or ADMIN - showing read-only." />}
      {error && <ErrorAlert message={error} />}

      <Card>
        <CardHeader title={`${teams.length} team${teams.length === 1 ? '' : 's'}`} />
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-xs uppercase text-neutral-500 dark:text-neutral-400">
              <tr>
                <th className="px-5 py-2.5 font-medium">Name</th>
                <th className="px-5 py-2.5 font-medium">Members</th>
                <th className="px-5 py-2.5 font-medium">Model access</th>
                <th className="px-5 py-2.5 font-medium">Monthly quota</th>
                <th className="px-5 py-2.5 font-medium">Content policy</th>
                <th className="px-5 py-2.5 font-medium">Created</th>
                <th className="px-5 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-950/5 dark:divide-white/10">
              {teams.map((team) => (
                <tr key={team.id} className="hover:bg-neutral-950/[0.02] dark:hover:bg-white/[0.02]">
                  <td className="px-5 py-3 text-neutral-900 dark:text-white">
                    {renamingId === team.id ? (
                      <div className="flex items-center gap-1.5">
                        <TextInput
                          autoFocus
                          value={renameValue}
                          onChange={(e) => setRenameValue(e.target.value)}
                          className="max-w-xs"
                        />
                        <Button variant="ghost" className="px-2" onClick={() => commitRename(team.id)}>
                          <CheckIcon className="h-4 w-4 text-emerald-600" />
                        </Button>
                        <Button variant="ghost" className="px-2" onClick={() => setRenamingId(null)}>
                          <XMarkIcon className="h-4 w-4" />
                        </Button>
                      </div>
                    ) : (
                      team.name
                    )}
                  </td>
                  <td className="px-5 py-3">
                    <button
                      type="button"
                      onClick={() => setViewingMembers(team)}
                      className="inline-flex items-center gap-1 rounded-md bg-neutral-100 px-2 py-0.5 text-xs font-medium text-neutral-600 hover:bg-neutral-200 dark:bg-white/5 dark:text-neutral-400 dark:hover:bg-white/10"
                    >
                      <UserGroupIcon className="h-3.5 w-3.5" />
                      {memberCounts[team.id] ?? '…'}
                    </button>
                  </td>
                  <td className="px-5 py-3">
                    {team.modelAllowlist.length === 0 ? (
                      <Badge tone="neutral">Unrestricted</Badge>
                    ) : (
                      <Badge tone="ok">{team.modelAllowlist.length} rule{team.modelAllowlist.length === 1 ? '' : 's'}</Badge>
                    )}
                  </td>
                  <td className="px-5 py-3">
                    {team.monthlyTokenBudget === null ? (
                      <Badge tone="neutral">Unlimited</Badge>
                    ) : (
                      <Badge tone="ok">{team.monthlyTokenBudget.toLocaleString()} tok/mo</Badge>
                    )}
                  </td>
                  <td className="px-5 py-3">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {team.policy.denyOnSensitiveData ? (
                        <Badge tone="alert">Deny on secret</Badge>
                      ) : (
                        <Badge tone="neutral">Redact on secret</Badge>
                      )}
                      {team.policy.blockedPatterns.length > 0 && (
                        <Badge tone="ok">
                          +{team.policy.blockedPatterns.length} blocked pattern{team.policy.blockedPatterns.length === 1 ? '' : 's'}
                        </Badge>
                      )}
                    </div>
                  </td>
                  <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{new Date(team.createdAt).toLocaleDateString()}</td>
                  <td className="px-5 py-3 text-right">
                    <div className="flex justify-end gap-1">
                      {isOwnerOrAdmin && (
                        <>
                          <Button variant="ghost" className="px-2" title="Model access" onClick={() => setEditingAllowlist(team)}>
                            <AdjustmentsHorizontalIcon className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" className="px-2" title="Quota & usage" onClick={() => setEditingQuota(team)}>
                            <ChartBarIcon className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" className="px-2" title="Content policy" onClick={() => setEditingPolicy(team)}>
                            <ShieldExclamationIcon className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" className="px-2" title="Rename" onClick={() => startRename(team)}>
                            <PencilIcon className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" className="px-2 text-red-600 dark:text-red-400" title="Delete" onClick={() => handleDelete(team)}>
                            <TrashIcon className="h-4 w-4" />
                          </Button>
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && teams.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-5 py-10 text-center text-sm text-neutral-400">
                    No teams yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <CreateTeamModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={async () => {
          setCreateOpen(false);
          await load();
        }}
      />
      <AllowlistModal
        team={editingAllowlist}
        onClose={() => setEditingAllowlist(null)}
        onSaved={async () => {
          setEditingAllowlist(null);
          await load();
        }}
      />
      <QuotaModal
        team={editingQuota}
        onClose={() => setEditingQuota(null)}
        onSaved={async () => {
          setEditingQuota(null);
          await load();
        }}
      />
      <PolicyModal
        team={editingPolicy}
        onClose={() => setEditingPolicy(null)}
        onSaved={async () => {
          setEditingPolicy(null);
          await load();
        }}
      />
      <MembersModal
        team={viewingMembers}
        canManage={isOwnerOrAdmin}
        onClose={() => setViewingMembers(null)}
        onMembershipChanged={load}
      />
    </div>
  );
}

function MembersModal({
  team,
  canManage,
  onClose,
  onMembershipChanged
}: {
  team: Team | null;
  canManage: boolean;
  onClose: () => void;
  onMembershipChanged: () => void;
}) {
  const [members, setMembers] = React.useState<Account[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [removingId, setRemovingId] = React.useState<string | null>(null);
  const [query, setQuery] = React.useState('');
  const [showAdd, setShowAdd] = React.useState(false);

  const load = React.useCallback(async () => {
    if (!team) return;
    setLoading(true);
    setError(null);
    try {
      setMembers(await listAccounts(team.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [team]);

  React.useEffect(() => {
    setQuery('');
    setShowAdd(false);
    load();
  }, [load]);

  if (!team) return null;

  const filtered = members.filter((m) => {
    const q = query.trim().toLowerCase();
    return !q || m.email.toLowerCase().includes(q) || m.username.toLowerCase().includes(q);
  });

  const refresh = async () => {
    await load();
    onMembershipChanged();
  };

  const handleRemove = async (account: Account) => {
    const remainingOwners = members.filter((m) => m.role === 'OWNER' && m.id !== account.id).length;
    if (account.role === 'OWNER' && remainingOwners === 0) {
      setError(`${account.username} is the only OWNER of ${team.name} - promote another member to OWNER first.`);
      return;
    }
    if (!confirm(`Remove ${account.username} from ${team.name}?`)) return;
    setRemovingId(account.id);
    setError(null);
    try {
      await updateAccount({ id: account.id, clearTeamId: true });
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setRemovingId(null);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Members - ${team.name}`}>
      <div className="space-y-3">
        <TextInput
          placeholder="Filter by email or username…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          autoFocus
        />
        {error && <ErrorAlert message={error} />}
        <div className="max-h-64 divide-y divide-neutral-950/5 overflow-y-auto dark:divide-white/10">
          {filtered.map((m) => (
            <div key={m.id} className="flex items-center justify-between gap-2 py-2">
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-neutral-900 dark:text-white">{m.username}</p>
                <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{m.email}</p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <RoleBadge role={m.role} />
                {canManage && (
                  <Button
                    variant="ghost"
                    className="px-2 text-red-600 dark:text-red-400"
                    title="Remove from team"
                    onClick={() => handleRemove(m)}
                    disabled={removingId === m.id}
                  >
                    <XMarkIcon className="h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          ))}
          {!loading && filtered.length === 0 && (
            <p className="py-6 text-center text-sm text-neutral-400">
              {members.length === 0 ? 'No members in this team yet.' : 'No members match this filter.'}
            </p>
          )}
          {loading && members.length === 0 && <p className="py-6 text-center text-sm text-neutral-400">Loading…</p>}
        </div>

        {canManage && (
          <div className="border-t border-neutral-950/5 pt-3 dark:border-white/10">
            {showAdd ? (
              <AddMemberForm
                teamId={team.id}
                onCancel={() => setShowAdd(false)}
                onAdded={async () => {
                  setShowAdd(false);
                  await refresh();
                }}
              />
            ) : (
              <Button variant="ghost" onClick={() => setShowAdd(true)}>
                <PlusIcon className="h-4 w-4" />
                Add member
              </Button>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

/**
 * Creates a brand-new MEMBER account directly on this team, rather than
 * reassigning an existing one - the accounts query has no cross-team
 * visibility (by design, since a recent hardening pass closed platform-wide
 * account reads), so there's no way to search for and pull in an existing
 * unaffiliated account from here. Promoting the new member past MEMBER, or
 * moving an existing account between teams, is still done from the Accounts
 * page.
 */
function AddMemberForm({ teamId, onCancel, onAdded }: { teamId: string; onCancel: () => void; onAdded: () => void }) {
  const [email, setEmail] = React.useState('');
  const [username, setUsername] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await createAccount({ email, username, teamId });
      onAdded();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <p className="text-xs text-neutral-400">
        Creates a new MEMBER account on this team. To add an existing account instead, change its team from the
        Accounts page.
      </p>
      <TextInput required type="email" placeholder="Email" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} />
      <TextInput required placeholder="Username" value={username} onChange={(e) => setUsername(e.target.value)} />
      {error && <ErrorAlert message={error} />}
      <div className="flex justify-end gap-2">
        <Button type="button" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" variant="primary" disabled={saving}>
          {saving ? 'Adding…' : 'Add member'}
        </Button>
      </div>
    </form>
  );
}

function CreateTeamModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [name, setName] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setName('');
      setError(null);
    }
  }, [open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await createTeam(name);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="New team">
      <form onSubmit={handleSubmit} className="space-y-3">
        <Field label="Name">
          <TextInput required autoFocus value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Creating…' : 'Create team'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function AllowlistModal({
  team,
  onClose,
  onSaved
}: {
  team: Team | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [text, setText] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  const [testProvider, setTestProvider] = React.useState('');
  const [testModel, setTestModel] = React.useState('');
  const [testResult, setTestResult] = React.useState<boolean | null>(null);
  const [testing, setTesting] = React.useState(false);

  React.useEffect(() => {
    if (team) {
      setText(team.modelAllowlist.join('\n'));
      setError(null);
      setTestProvider('');
      setTestModel('');
      setTestResult(null);
    }
  }, [team]);

  if (!team) return null;

  const entries = () =>
    text
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await updateTeamModelAllowlist(team.id, entries());
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!testProvider.trim() || !testModel.trim()) return;
    setTesting(true);
    setTestResult(null);
    try {
      setTestResult(await isModelAllowed(team.id, testProvider.trim(), testModel.trim()));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setTesting(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Model access - ${team.name}`}>
      <form onSubmit={handleSubmit} className="space-y-3">
        <Field label="Allowlist (one entry per line, e.g. anthropic:*, openai:gpt-4o)">
          <Textarea rows={5} value={text} onChange={(e) => setText(e.target.value)} placeholder="Leave empty for unrestricted" />
        </Field>
        <p className="text-xs text-neutral-400">
          Enforced on every /v1/chat/completions call - a model not on this list is rejected before it reaches a provider.
        </p>
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save allowlist'}
          </Button>
        </div>
      </form>

      <div className="mt-5 border-t border-neutral-950/5 pt-4 dark:border-white/10">
        <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300">Test a provider/model pair</p>
        <div className="mt-2 flex items-end gap-2">
          <TextInput placeholder="provider (e.g. anthropic)" value={testProvider} onChange={(e) => setTestProvider(e.target.value)} />
          <TextInput placeholder="model (e.g. claude-opus)" value={testModel} onChange={(e) => setTestModel(e.target.value)} />
          <Button onClick={handleTest} disabled={testing || !testProvider.trim() || !testModel.trim()}>
            {testing ? 'Checking…' : 'Check'}
          </Button>
        </div>
        {testResult !== null && (
          <p className="mt-2 text-sm">
            {testResult ? <Badge tone="ok">Allowed</Badge> : <Badge tone="alert">Denied</Badge>}
          </p>
        )}
      </div>
    </Modal>
  );
}

function QuotaModal({ team, onClose, onSaved }: { team: Team | null; onClose: () => void; onSaved: () => void }) {
  const [unlimited, setUnlimited] = React.useState(true);
  const [budget, setBudget] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [usage, setUsage] = React.useState<UsageSummary | null>(null);
  const [usageError, setUsageError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!team) return;
    setUnlimited(team.monthlyTokenBudget === null);
    setBudget(team.monthlyTokenBudget === null ? '' : String(team.monthlyTokenBudget));
    setError(null);
    setUsage(null);
    setUsageError(null);
    getTeamUsage(team.id)
      .then(setUsage)
      .catch((err) => setUsageError(err instanceof Error ? err.message : String(err)));
  }, [team]);

  if (!team) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!unlimited && (!budget.trim() || Number(budget) <= 0)) {
      setError('Enter a positive token budget, or check "Unlimited".');
      return;
    }
    setSaving(true);
    try {
      await updateTeamQuota(team.id, unlimited ? null : Number(budget));
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Quota & usage - ${team.name}`}>
      <div className="space-y-1.5">
        <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300">This calendar month</p>
        {usageError && <ErrorAlert message={usageError} />}
        {!usageError && (
          <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <UsageStat label="Requests" value={usage ? usage.requestCount.toLocaleString() : '…'} />
            <UsageStat label="Total tokens" value={usage ? usage.totalTokens.toLocaleString() : '…'} />
            <UsageStat label="Prompt / completion" value={usage ? `${usage.promptTokens.toLocaleString()} / ${usage.completionTokens.toLocaleString()}` : '…'} />
            <UsageStat label="Est. cost" value={usage ? `$${usage.costUsd.toFixed(4)}` : '…'} />
          </dl>
        )}
      </div>

      <form onSubmit={handleSubmit} className="mt-5 space-y-3 border-t border-neutral-950/5 pt-4 dark:border-white/10">
        <p className="text-sm font-medium text-neutral-700 dark:text-neutral-300">Monthly token budget</p>
        <label className="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-300">
          <input
            type="checkbox"
            checked={unlimited}
            onChange={(e) => setUnlimited(e.target.checked)}
            className="h-4 w-4 rounded border-neutral-300 text-neutral-900 dark:border-white/20"
          />
          Unlimited
        </label>
        {!unlimited && (
          <Field label="Tokens per calendar month (prompt + completion, summed across the team's keys)">
            <TextInput
              type="number"
              min={1}
              value={budget}
              onChange={(e) => setBudget(e.target.value)}
              placeholder="e.g. 1000000"
            />
          </Field>
        )}
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save quota'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function PolicyModal({ team, onClose, onSaved }: { team: Team | null; onClose: () => void; onSaved: () => void }) {
  const [text, setText] = React.useState('');
  const [denyOnSensitiveData, setDenyOnSensitiveData] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (team) {
      setText(team.policy.blockedPatterns.join('\n'));
      setDenyOnSensitiveData(team.policy.denyOnSensitiveData);
      setError(null);
    }
  }, [team]);

  if (!team) return null;

  const entries = () =>
    text
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await updateTeamPolicy(team.id, entries(), denyOnSensitiveData);
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Content policy - ${team.name}`}>
      <form onSubmit={handleSubmit} className="space-y-3">
        <p className="text-xs text-neutral-400">
          These overrides only add to the platform baseline (a prompt-size limit and secret/PII redaction that apply
          to every team) - a team can make its policy stricter here, never weaker.
        </p>
        <Field label="Additional blocked keywords/phrases (one per line, case-insensitive)">
          <Textarea rows={4} value={text} onChange={(e) => setText(e.target.value)} placeholder="Leave empty for none" />
        </Field>
        <label className="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-300">
          <input
            type="checkbox"
            checked={denyOnSensitiveData}
            onChange={(e) => setDenyOnSensitiveData(e.target.checked)}
            className="h-4 w-4 rounded border-neutral-300 text-neutral-900 dark:border-white/20"
          />
          Deny requests that look like they contain a secret (API key, credit card number), instead of redacting and
          continuing
        </label>
        {error && <ErrorAlert message={error} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save policy'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function UsageStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs uppercase text-neutral-400">{label}</dt>
      <dd className="mt-0.5 text-sm font-medium text-neutral-900 dark:text-white">{value}</dd>
    </div>
  );
}
