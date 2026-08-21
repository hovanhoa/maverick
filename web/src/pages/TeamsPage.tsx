import * as React from 'react';
import { PlusIcon, PencilIcon, TrashIcon, CheckIcon, XMarkIcon, AdjustmentsHorizontalIcon } from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { Modal } from '../components/ui/Modal';
import { Field, TextInput, Textarea } from '../components/ui/Field';
import { listTeams, createTeam, updateTeam, deleteTeam, updateTeamModelAllowlist, isModelAllowed } from '../lib/teams';
import { useAuth } from '../lib/auth';
import type { Team } from '../lib/api';

export function TeamsPage() {
  const { isOwnerOrAdmin } = useAuth();
  const [teams, setTeams] = React.useState<Team[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [renamingId, setRenamingId] = React.useState<string | null>(null);
  const [renameValue, setRenameValue] = React.useState('');
  const [editingAllowlist, setEditingAllowlist] = React.useState<Team | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setTeams(await listTeams());
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
                <th className="px-5 py-2.5 font-medium">Model access</th>
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
                    {team.modelAllowlist.length === 0 ? (
                      <Badge tone="neutral">Unrestricted</Badge>
                    ) : (
                      <Badge tone="ok">{team.modelAllowlist.length} rule{team.modelAllowlist.length === 1 ? '' : 's'}</Badge>
                    )}
                  </td>
                  <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{new Date(team.createdAt).toLocaleDateString()}</td>
                  <td className="px-5 py-3 text-right">
                    <div className="flex justify-end gap-1">
                      {isOwnerOrAdmin && (
                        <>
                          <Button variant="ghost" className="px-2" title="Model access" onClick={() => setEditingAllowlist(team)}>
                            <AdjustmentsHorizontalIcon className="h-4 w-4" />
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
                  <td colSpan={4} className="px-5 py-10 text-center text-sm text-neutral-400">
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
    </div>
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
          No proxy path enforces this yet (that's Phase 3) - this configures the allowlist ahead of it.
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
