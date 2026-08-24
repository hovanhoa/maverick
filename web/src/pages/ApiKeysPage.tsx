import * as React from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { PlusIcon, NoSymbolIcon, ClipboardDocumentIcon, PlayIcon, Cog6ToothIcon } from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { Field, Select, TextInput } from '../components/ui/Field';
import { Modal } from '../components/ui/Modal';
import { listAccounts } from '../lib/accounts';
import { listApiKeys, createApiKey, revokeApiKey, updateApiKeyQuota } from '../lib/apikeys';
import { useAuth } from '../lib/auth';
import type { Account, ApiKey } from '../lib/api';

/** Formats an ISO timestamp as relative time (e.g. "3h ago"), or "Never" for null. */
function formatLastUsed(iso: string | null): string {
  if (!iso) return 'Never';
  const diffMs = Date.now() - new Date(iso).getTime();
  const minutes = Math.round(diffMs / 60_000);
  if (minutes < 1) return 'Just now';
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  return `${days}d ago`;
}

export function ApiKeysPage() {
  const { account: me, isOwnerOrAdmin } = useAuth();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const accountId = searchParams.get('accountId') ?? '';

  const [accounts, setAccounts] = React.useState<Account[]>([]);
  const [keys, setKeys] = React.useState<ApiKey[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [freshKey, setFreshKey] = React.useState<string | null>(null);
  const [copied, setCopied] = React.useState(false);
  const [issuing, setIssuing] = React.useState(false);
  const [editingQuota, setEditingQuota] = React.useState<ApiKey | null>(null);

  React.useEffect(() => {
    listAccounts()
      .then(setAccounts)
      .catch((err) => setError(err instanceof Error ? err.message : String(err)));
  }, []);

  const load = React.useCallback(async (id: string) => {
    if (!id) {
      setKeys([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setKeys(await listApiKeys(id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load(accountId);
  }, [accountId, load]);

  const selectAccount = (id: string) => {
    setFreshKey(null);
    setSearchParams(id ? { accountId: id } : {});
  };

  // Default to the signed-in account when nothing is selected yet - most
  // visits are "show me my own keys", even for an OWNER/ADMIN who could
  // pick someone else's via the dropdown.
  React.useEffect(() => {
    if (!accountId && me) {
      selectAccount(me.id);
    }
  }, [accountId, me]);

  const manageableAccounts = isOwnerOrAdmin ? accounts : accounts.filter((a) => a.id === me?.id);

  const handleIssue = async () => {
    if (!accountId) return;
    setIssuing(true);
    setError(null);
    try {
      const secret = await createApiKey(accountId);
      setFreshKey(secret.key);
      setCopied(false);
      await load(accountId);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setIssuing(false);
    }
  };

  const handleCopy = () => {
    if (!freshKey) return;
    navigator.clipboard.writeText(freshKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const handleRevoke = async (key: ApiKey) => {
    if (!confirm(`Revoke key ${key.prefix}…? Anything using it will lose access immediately.`)) return;
    setError(null);
    try {
      await revokeApiKey(key.id);
      await load(accountId);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const selectedAccount = accounts.find((a) => a.id === accountId);

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">API Keys</h1>
        <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">Pick an account to view and manage its keys.</p>
      </div>

      <Card className="p-5">
        <Field label="Account">
          {isOwnerOrAdmin ? (
            <Select value={accountId} onChange={(e) => selectAccount(e.target.value)}>
              <option value="">Select an account…</option>
              {manageableAccounts.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.email} ({a.role})
                </option>
              ))}
            </Select>
          ) : (
            <div className="flex items-center justify-between rounded-md bg-neutral-50 px-3 py-2 text-sm text-neutral-600 ring-1 ring-inset ring-neutral-200 dark:bg-white/5 dark:text-neutral-300 dark:ring-white/10">
              {me?.email ?? '…'}
              <span className="text-xs text-neutral-400">Self only</span>
            </div>
          )}
        </Field>
        {!isOwnerOrAdmin && (
          <p className="mt-2 text-xs text-neutral-400">Managing another account's keys requires OWNER or ADMIN.</p>
        )}
      </Card>

      {error && <ErrorAlert message={error} />}

      {freshKey && (
        <Card className="border-emerald-200 bg-emerald-50/60 p-4 ring-emerald-200 dark:bg-emerald-500/10 dark:ring-emerald-500/20">
          <p className="text-sm font-medium text-emerald-800 dark:text-emerald-400">
            Key created - store it now, it won't be shown again.
          </p>
          <div className="mt-2.5 flex items-center gap-2">
            <code className="flex-1 truncate rounded-md bg-white px-3 py-2 text-sm text-neutral-900 ring-1 ring-inset ring-neutral-200 dark:bg-neutral-950 dark:text-white dark:ring-white/10">
              {freshKey}
            </code>
            <Button onClick={handleCopy} title="Copy to clipboard">
              <ClipboardDocumentIcon className="h-4 w-4" />
              {copied ? 'Copied' : 'Copy'}
            </Button>
            <Button onClick={() => navigate('/playground', { state: { apiKey: freshKey } })} title="Test this key in the Playground">
              <PlayIcon className="h-4 w-4" />
              Test
            </Button>
          </div>
        </Card>
      )}

      {accountId && (
        <Card>
          <CardHeader
            title={selectedAccount ? `Keys for ${selectedAccount.email}` : 'Keys'}
            action={
              <Button variant="primary" onClick={handleIssue} disabled={issuing}>
                <PlusIcon className="h-4 w-4" />
                {issuing ? 'Issuing…' : 'Issue new key'}
              </Button>
            }
          />
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-neutral-500 dark:text-neutral-400">
                <tr>
                  <th className="px-5 py-2.5 font-medium">Prefix</th>
                  <th className="px-5 py-2.5 font-medium">Created</th>
                  <th className="px-5 py-2.5 font-medium">Last used</th>
                  <th className="px-5 py-2.5 font-medium">Budget</th>
                  <th className="px-5 py-2.5 font-medium">Status</th>
                  <th className="px-5 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-950/5 dark:divide-white/10">
                {keys.map((key) => (
                  <tr key={key.id} className="hover:bg-neutral-950/[0.02] dark:hover:bg-white/[0.02]">
                    <td className="px-5 py-3 font-mono text-neutral-900 dark:text-white">{key.prefix}…</td>
                    <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{new Date(key.createdAt).toLocaleString()}</td>
                    <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400" title={key.lastUsedAt ? new Date(key.lastUsedAt).toLocaleString() : undefined}>
                      {formatLastUsed(key.lastUsedAt)}
                    </td>
                    <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">
                      {key.monthlyTokenBudget === null ? 'Unlimited' : <Badge tone="ok">{key.monthlyTokenBudget.toLocaleString()} tok/mo</Badge>}
                    </td>
                    <td className="px-5 py-3">
                      {key.revokedAt ? <Badge tone="alert">Revoked</Badge> : <Badge tone="ok">Active</Badge>}
                    </td>
                    <td className="px-5 py-3 text-right">
                      <div className="flex justify-end gap-1">
                        {isOwnerOrAdmin && !key.revokedAt && (
                          <Button variant="ghost" className="px-2" title="Quota" onClick={() => setEditingQuota(key)}>
                            <Cog6ToothIcon className="h-4 w-4" />
                          </Button>
                        )}
                        {!key.revokedAt && (
                          <Button variant="ghost" className="px-2 text-red-600 dark:text-red-400" title="Revoke" onClick={() => handleRevoke(key)}>
                            <NoSymbolIcon className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
                {!loading && keys.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-5 py-10 text-center text-sm text-neutral-400">
                      No keys issued yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      <ApiKeyQuotaModal
        apiKey={editingQuota}
        onClose={() => setEditingQuota(null)}
        onSaved={() => {
          setEditingQuota(null);
          load(accountId);
        }}
      />
    </div>
  );
}

function ApiKeyQuotaModal({ apiKey, onClose, onSaved }: { apiKey: ApiKey | null; onClose: () => void; onSaved: () => void }) {
  const [unlimited, setUnlimited] = React.useState(true);
  const [budget, setBudget] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!apiKey) return;
    setUnlimited(apiKey.monthlyTokenBudget === null);
    setBudget(apiKey.monthlyTokenBudget === null ? '' : String(apiKey.monthlyTokenBudget));
    setError(null);
  }, [apiKey]);

  if (!apiKey) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!unlimited && (!budget.trim() || Number(budget) <= 0)) {
      setError('Enter a positive token budget, or check "Unlimited".');
      return;
    }
    setSaving(true);
    try {
      await updateApiKeyQuota(apiKey.id, unlimited ? null : Number(budget));
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Quota - ${apiKey.prefix}…`}>
      <form onSubmit={handleSubmit} className="space-y-3">
        <p className="text-sm text-neutral-500 dark:text-neutral-400">
          Caps this one key's usage, on top of whatever budget its account and team may also have.
        </p>
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
          <Field label="Tokens per calendar month (prompt + completion)">
            <TextInput type="number" min={1} value={budget} onChange={(e) => setBudget(e.target.value)} placeholder="e.g. 100000" />
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
