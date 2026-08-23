import * as React from 'react';
import { ChevronRightIcon, ChevronDownIcon } from '@heroicons/react/24/outline';
import { Card, CardHeader } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { ErrorAlert } from '../components/ui/Alert';
import { useAuth } from '../lib/auth';
import { getMyRequestLogs, getTeamRequestLogs, getGlobalRequestLogs } from '../lib/requestLogs';
import type { RequestLog, RequestLogConnection } from '../lib/api';

const PAGE_SIZE = 20;

function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function StatusBadge({ status }: { status: RequestLog['status'] }) {
  return <Badge tone={status === 'SUCCESS' ? 'ok' : 'alert'}>{status}</Badge>;
}

function RequestLogRow({ entry }: { entry: RequestLog }) {
  const [expanded, setExpanded] = React.useState(false);

  return (
    <>
      <tr
        className="cursor-pointer hover:bg-neutral-950/[0.02] dark:hover:bg-white/[0.02]"
        onClick={() => setExpanded((v) => !v)}
      >
        <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">
          {expanded ? <ChevronDownIcon className="h-4 w-4" /> : <ChevronRightIcon className="h-4 w-4" />}
        </td>
        <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{new Date(entry.createdAt).toLocaleString()}</td>
        <td className="px-5 py-3 text-neutral-900 dark:text-white">{entry.provider ? `${entry.provider}/${entry.model}` : entry.requestedModel}</td>
        <td className="px-5 py-3">
          <StatusBadge status={entry.status} />
        </td>
        <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{entry.latencyMs} ms</td>
        <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{entry.totalTokens ?? '—'}</td>
        <td className="px-5 py-3 text-neutral-500 dark:text-neutral-400">{entry.costUsd != null ? `$${entry.costUsd.toFixed(4)}` : '—'}</td>
      </tr>
      {expanded && (
        <tr>
          <td colSpan={7} className="bg-neutral-50 px-5 py-4 dark:bg-black/20">
            <div className="space-y-3">
              {entry.errorMessage && (
                <p className="text-sm text-red-600 dark:text-red-400">
                  {entry.errorKind && <span className="font-medium">{entry.errorKind}: </span>}
                  {entry.errorMessage}
                </p>
              )}
              <div>
                <p className="mb-1 text-xs font-medium uppercase text-neutral-500 dark:text-neutral-400">Request</p>
                <pre className="max-h-64 overflow-auto rounded-md bg-neutral-900 p-3 text-xs text-neutral-100">
                  {formatJSON(entry.requestBody)}
                </pre>
              </div>
              {entry.responseBody ? (
                <div>
                  <p className="mb-1 text-xs font-medium uppercase text-neutral-500 dark:text-neutral-400">Response</p>
                  <pre className="max-h-64 overflow-auto rounded-md bg-neutral-900 p-3 text-xs text-neutral-100">
                    {formatJSON(entry.responseBody)}
                  </pre>
                </div>
              ) : (
                <p className="text-xs text-neutral-400">
                  {entry.stream ? 'Streamed response - not captured.' : 'No response body.'}
                </p>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

/** A self-contained, paginated request log table backed by one fetcher (self/team/global). */
function RequestLogTable({
  title,
  description,
  fetcher
}: {
  title: string;
  description?: string;
  fetcher: (limit: number, offset: number) => Promise<RequestLogConnection>;
}) {
  const [conn, setConn] = React.useState<RequestLogConnection | null>(null);
  const [offset, setOffset] = React.useState(0);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetcher(PAGE_SIZE, offset)
      .then((result) => {
        if (!cancelled) setConn(result);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [fetcher, offset]);

  const items = conn?.items ?? [];
  const totalCount = conn?.totalCount ?? 0;
  const rangeStart = totalCount === 0 ? 0 : offset + 1;
  const rangeEnd = Math.min(offset + items.length, totalCount);

  return (
    <Card>
      <CardHeader
        title={title}
        description={description}
        action={
          <div className="flex items-center gap-3">
            <span className="text-xs text-neutral-500 dark:text-neutral-400">
              {totalCount > 0 ? `${rangeStart}-${rangeEnd} of ${totalCount}` : loading ? 'Loading…' : 'No calls yet'}
            </span>
            <Button variant="secondary" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
              Previous
            </Button>
            <Button variant="secondary" disabled={!conn?.hasNextPage} onClick={() => setOffset(offset + PAGE_SIZE)}>
              Next
            </Button>
          </div>
        }
      />
      {error && (
        <div className="px-5 py-4">
          <ErrorAlert message={error} />
        </div>
      )}
      {items.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-xs uppercase text-neutral-500 dark:text-neutral-400">
              <tr>
                <th className="px-5 py-2.5" />
                <th className="px-5 py-2.5 font-medium">Time</th>
                <th className="px-5 py-2.5 font-medium">Model</th>
                <th className="px-5 py-2.5 font-medium">Status</th>
                <th className="px-5 py-2.5 font-medium">Latency</th>
                <th className="px-5 py-2.5 font-medium">Tokens</th>
                <th className="px-5 py-2.5 font-medium">Cost</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-950/5 dark:divide-white/10">
              {items.map((entry) => (
                <RequestLogRow key={entry.id} entry={entry} />
              ))}
            </tbody>
          </table>
        </div>
      )}
      {!loading && items.length === 0 && !error && (
        <p className="px-5 py-6 text-center text-sm text-neutral-400">No requests recorded yet.</p>
      )}
    </Card>
  );
}

export function RequestLogsPage() {
  const { account: me, isOwnerOrAdmin } = useAuth();
  const teamId = me?.teamId ?? null;

  const myFetcher = React.useCallback((limit: number, offset: number) => getMyRequestLogs(limit, offset), []);
  const teamFetcher = React.useCallback(
    (limit: number, offset: number) => getTeamRequestLogs(teamId!, limit, offset),
    [teamId]
  );
  const globalFetcher = React.useCallback((limit: number, offset: number) => getGlobalRequestLogs(limit, offset), []);

  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-xl font-semibold text-neutral-900 dark:text-white">Request logs</h1>
        <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
          Every LLM proxy call attempt, including ones blocked before reaching a provider, with the full request and
          (for non-streaming calls) response content. Click a row to expand it.
        </p>
      </div>

      <RequestLogTable title="My request log" description="Your own proxy calls" fetcher={myFetcher} />

      {teamId && isOwnerOrAdmin && (
        <RequestLogTable
          title="Team request log"
          description="Every member's calls - visible to team OWNER/ADMIN only"
          fetcher={teamFetcher}
        />
      )}

      {isOwnerOrAdmin && (
        <RequestLogTable
          title="Platform-wide request log"
          description="Every team and account, OWNER/ADMIN only"
          fetcher={globalFetcher}
        />
      )}
    </div>
  );
}
