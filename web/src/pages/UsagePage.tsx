import * as React from 'react';
import { Card, CardHeader } from '../components/ui/Card';
import { ErrorAlert } from '../components/ui/Alert';
import { LineChart, BarList } from '../components/ui/Chart';
import { Select, TextInput } from '../components/ui/Field';
import { useAuth } from '../lib/auth';
import { listAccounts } from '../lib/accounts';
import { listTeams } from '../lib/teams';
import { getTeamUsage } from '../lib/teams';
import {
  getMyUsage,
  getTeamUsageByAccount,
  getTeamUsageByModel,
  getTeamUsageDaily,
  getGlobalUsage,
  getGlobalUsageByTeam,
  type UsageWindow
} from '../lib/usage';
import type { Account, AccountUsage, DailyUsage, ModelUsage, Team, TeamUsage, UsageSummary } from '../lib/api';

type RangeKey = 'month' | '7d' | '30d' | '90d' | 'all' | 'custom';

const RANGE_OPTIONS: { key: RangeKey; label: string }[] = [
  { key: 'month', label: 'This month' },
  { key: '7d', label: 'Last 7 days' },
  { key: '30d', label: 'Last 30 days' },
  { key: '90d', label: 'Last 90 days' },
  { key: 'all', label: 'All time' },
  { key: 'custom', label: 'Custom range…' }
];

/** Resolves a preset range to a since/until window. 'month' omits since to keep the server's default (start of the current calendar month). 'custom' is resolved separately, from the from/until date inputs. */
function windowForRange(range: RangeKey): UsageWindow {
  const now = new Date();
  switch (range) {
    case '7d':
      return { since: new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString() };
    case '30d':
      return { since: new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000).toISOString() };
    case '90d':
      return { since: new Date(now.getTime() - 90 * 24 * 60 * 60 * 1000).toISOString() };
    case 'all':
      return { since: new Date(0).toISOString() };
    case 'month':
    case 'custom':
    default:
      return {};
  }
}

/** Resolves the two <input type="date"> values for a custom range into a since/until window, swapping them if entered backwards. Each bound covers the full local calendar day. */
function customWindow(from: string, until: string): UsageWindow {
  if (from && until && from > until) {
    [from, until] = [until, from];
  }
  return {
    since: from ? new Date(`${from}T00:00:00.000Z`).toISOString() : undefined,
    until: until ? new Date(`${until}T23:59:59.999Z`).toISOString() : undefined
  };
}

const formatCost = (usd: number) => `$${usd.toFixed(usd < 1 ? 4 : 2)}`;
const formatNumber = (n: number) => n.toLocaleString();

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <Card className="px-5 py-4">
      <p className="text-xs font-medium uppercase text-neutral-500 dark:text-neutral-400">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-neutral-900 dark:text-white">{value}</p>
      {hint && <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">{hint}</p>}
    </Card>
  );
}

function SummaryStats({ summary }: { summary: UsageSummary }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <StatCard label="Requests" value={formatNumber(summary.requestCount)} />
      <StatCard label="Tokens" value={formatNumber(summary.totalTokens)} hint={`${formatNumber(summary.promptTokens)} in / ${formatNumber(summary.completionTokens)} out`} />
      <StatCard label="Cost" value={formatCost(summary.costUsd)} hint="Estimated, illustrative pricing" />
    </div>
  );
}

function BudgetBar({ used, budget }: { used: number; budget: number }) {
  const pct = budget > 0 ? Math.min(100, Math.round((used / budget) * 100)) : 0;
  const over = used > budget;
  return (
    <div>
      <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
        <span>Monthly token budget</span>
        <span>
          {formatNumber(used)} / {formatNumber(budget)} ({pct}%)
        </span>
      </div>
      <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-neutral-100 dark:bg-white/10">
        <div
          className={`h-full rounded-full ${over ? 'bg-red-500' : 'bg-primary-600'}`}
          style={{ width: `${Math.min(100, pct)}%` }}
        />
      </div>
    </div>
  );
}

const formatShortDate = (iso: string) => new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });

function DailyTrend({ points }: { points: DailyUsage[] }) {
  return (
    <LineChart
      points={points.map((p) => ({
        x: p.date,
        y: p.costUsd,
        meta: [{ label: 'Requests', value: formatNumber(p.requestCount) }, { label: 'Tokens', value: formatNumber(p.totalTokens) }]
      }))}
      formatY={formatCost}
      formatX={formatShortDate}
    />
  );
}

export function UsagePage() {
  const { account: me, isOwnerOrAdmin } = useAuth();
  const teamId = me?.teamId ?? null;

  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  const [myUsage, setMyUsage] = React.useState<UsageSummary | null>(null);
  const [team, setTeam] = React.useState<Team | null>(null);
  const [teamUsage, setTeamUsage] = React.useState<UsageSummary | null>(null);
  const [byAccount, setByAccount] = React.useState<AccountUsage[]>([]);
  const [accounts, setAccounts] = React.useState<Account[]>([]);
  const [byModel, setByModel] = React.useState<ModelUsage[]>([]);
  const [daily, setDaily] = React.useState<DailyUsage[]>([]);

  const [globalUsage, setGlobalUsage] = React.useState<UsageSummary | null>(null);
  const [globalByTeam, setGlobalByTeam] = React.useState<TeamUsage[]>([]);

  const [range, setRange] = React.useState<RangeKey>('month');
  const [customFrom, setCustomFrom] = React.useState('');
  const [customUntil, setCustomUntil] = React.useState('');
  const [modelFilter, setModelFilter] = React.useState('');
  const [memberFilter, setMemberFilter] = React.useState('');
  const usageWindow = React.useMemo(
    () => (range === 'custom' ? customWindow(customFrom, customUntil) : windowForRange(range)),
    [range, customFrom, customUntil]
  );

  const load = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mine = await getMyUsage(usageWindow);
      setMyUsage(mine);

      if (teamId) {
        const teamPromises: [Promise<Team[]>, Promise<UsageSummary>, Promise<ModelUsage[]>, Promise<DailyUsage[]>] = [
          listTeams(),
          getTeamUsage(teamId, usageWindow.since),
          getTeamUsageByModel(teamId, usageWindow),
          getTeamUsageDaily(teamId, usageWindow)
        ];
        const [allTeams, tUsage, models, days] = await Promise.all(teamPromises);
        setTeam(allTeams.find((t) => t.id === teamId) ?? null);
        setTeamUsage(tUsage);
        setByModel(models);
        setDaily(days);

        if (isOwnerOrAdmin) {
          const [members, accs] = await Promise.all([getTeamUsageByAccount(teamId, usageWindow), listAccounts()]);
          setByAccount(members);
          setAccounts(accs);
        }
      }

      if (isOwnerOrAdmin) {
        const [gUsage, gByTeam] = await Promise.all([getGlobalUsage(usageWindow), getGlobalUsageByTeam(usageWindow)]);
        setGlobalUsage(gUsage);
        setGlobalByTeam(gByTeam);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [teamId, isOwnerOrAdmin, usageWindow]);

  React.useEffect(() => {
    load();
  }, [load]);

  const accountLabel = (accountId: string) => accounts.find((a) => a.id === accountId)?.email ?? accountId;

  const filteredByModel = byModel.filter((row) => {
    const q = modelFilter.trim().toLowerCase();
    return !q || row.model.toLowerCase().includes(q) || row.provider.toLowerCase().includes(q);
  });
  const filteredByAccount = byAccount.filter((row) => {
    const q = memberFilter.trim().toLowerCase();
    return !q || accountLabel(row.accountId).toLowerCase().includes(q);
  });

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">Usage</h1>
          <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
            {range === 'month'
              ? 'LLM proxy usage for the current calendar month.'
              : range === 'custom' && customFrom && customUntil
                ? `LLM proxy usage from ${new Date(customFrom).toLocaleDateString()} to ${new Date(customUntil).toLocaleDateString()}.`
                : 'LLM proxy usage for the selected period.'}
          </p>
        </div>
        <div className="flex flex-wrap items-end gap-2">
          {range === 'custom' && (
            <>
              <div className="w-40">
                <TextInput
                  type="date"
                  value={customFrom}
                  onChange={(e) => setCustomFrom(e.target.value)}
                  max={customUntil || undefined}
                  aria-label="From date"
                />
              </div>
              <div className="w-40">
                <TextInput
                  type="date"
                  value={customUntil}
                  onChange={(e) => setCustomUntil(e.target.value)}
                  min={customFrom || undefined}
                  aria-label="Until date"
                />
              </div>
            </>
          )}
          <div className="w-40">
            <Select value={range} onChange={(e) => setRange(e.target.value as RangeKey)} aria-label="Reporting period">
              {RANGE_OPTIONS.map((opt) => (
                <option key={opt.key} value={opt.key}>
                  {opt.label}
                </option>
              ))}
            </Select>
          </div>
        </div>
      </div>

      {error && <ErrorAlert message={error} />}

      <Card>
        <CardHeader title="My usage" />
        <div className="px-5 py-4">{myUsage ? <SummaryStats summary={myUsage} /> : loading && <p className="text-sm text-neutral-400">Loading…</p>}</div>
      </Card>

      {teamId && (
        <Card>
          <CardHeader title={team ? `${team.name} team usage` : 'Team usage'} />
          <div className="space-y-4 px-5 py-4">
            {teamUsage && <SummaryStats summary={teamUsage} />}
            {teamUsage && team?.monthlyTokenBudget != null && <BudgetBar used={teamUsage.totalTokens} budget={team.monthlyTokenBudget} />}
          </div>
        </Card>
      )}

      {teamId && (
        <Card>
          <CardHeader title="Daily trend" description="Cost per day, this period" />
          <DailyTrend points={daily} />
        </Card>
      )}

      {teamId && byModel.length > 0 && (
        <Card>
          <CardHeader
            title="Usage by model"
            description="Ranked by cost"
            action={
              <div className="w-48">
                <TextInput
                  placeholder="Filter by provider/model…"
                  value={modelFilter}
                  onChange={(e) => setModelFilter(e.target.value)}
                />
              </div>
            }
          />
          {filteredByModel.length > 0 ? (
            <BarList
              items={filteredByModel.map((row) => ({
                key: `${row.provider}/${row.model}`,
                label: row.model,
                value: row.costUsd,
                meta: `${row.provider} · ${formatNumber(row.requestCount)} req · ${formatNumber(row.totalTokens)} tokens`
              }))}
              formatValue={formatCost}
            />
          ) : (
            <p className="px-5 py-6 text-center text-sm text-neutral-400">No models match this filter.</p>
          )}
        </Card>
      )}

      {teamId && isOwnerOrAdmin && byAccount.length > 0 && (
        <Card>
          <CardHeader
            title="Usage by member"
            description="Ranked by cost - visible to team OWNER/ADMIN only"
            action={
              <div className="w-48">
                <TextInput
                  placeholder="Filter by email…"
                  value={memberFilter}
                  onChange={(e) => setMemberFilter(e.target.value)}
                />
              </div>
            }
          />
          {filteredByAccount.length > 0 ? (
            <BarList
              items={filteredByAccount.map((row) => ({
                key: row.accountId,
                label: accountLabel(row.accountId),
                value: row.costUsd,
                meta: `${formatNumber(row.requestCount)} req · ${formatNumber(row.totalTokens)} tokens`
              }))}
              formatValue={formatCost}
            />
          ) : (
            <p className="px-5 py-6 text-center text-sm text-neutral-400">No members match this filter.</p>
          )}
        </Card>
      )}

      {isOwnerOrAdmin && globalUsage && (
        <Card>
          <CardHeader title="Platform-wide usage" description="Every team and account, OWNER/ADMIN only" />
          <div className="px-5 py-4">
            <SummaryStats summary={globalUsage} />
          </div>
          {globalByTeam.length > 0 && (
            <BarList
              items={globalByTeam.map((row) => ({
                key: row.teamId,
                label: row.name,
                value: row.costUsd,
                meta: `${formatNumber(row.requestCount)} req · ${formatNumber(row.totalTokens)} tokens`
              }))}
              formatValue={formatCost}
            />
          )}
        </Card>
      )}
    </div>
  );
}
