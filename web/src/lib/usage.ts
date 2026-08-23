import { gql, type AccountUsage, type DailyUsage, type ModelUsage, type TeamUsage, type UsageSummary } from './api';

const USAGE_SUMMARY_FIELDS = `requestCount promptTokens completionTokens totalTokens costUsd`;

/** Common shape for an optional [since, until] reporting window. Omitted bounds fall back to each query's server-side default. */
export interface UsageWindow {
  since?: string;
  until?: string;
}

export async function getMyUsage(window: UsageWindow = {}): Promise<UsageSummary> {
  const data = await gql<{ myUsage: UsageSummary }>(
    `query($since: Time) { myUsage(since: $since) { ${USAGE_SUMMARY_FIELDS} } }`,
    { since: window.since }
  );
  return data.myUsage;
}

export async function getAccountUsage(accountId: string, window: UsageWindow = {}): Promise<UsageSummary> {
  const data = await gql<{ accountUsage: UsageSummary }>(
    `query($accountId: ID!, $since: Time) { accountUsage(accountId: $accountId, since: $since) { ${USAGE_SUMMARY_FIELDS} } }`,
    { accountId, since: window.since }
  );
  return data.accountUsage;
}

export async function getTeamUsageByAccount(teamId: string, window: UsageWindow = {}): Promise<AccountUsage[]> {
  const data = await gql<{ teamUsageByAccount: AccountUsage[] }>(
    `query($teamId: ID!, $since: Time) { teamUsageByAccount(teamId: $teamId, since: $since) { accountId ${USAGE_SUMMARY_FIELDS} } }`,
    { teamId, since: window.since }
  );
  return data.teamUsageByAccount;
}

export async function getTeamUsageByModel(teamId: string, window: UsageWindow = {}): Promise<ModelUsage[]> {
  const data = await gql<{ teamUsageByModel: ModelUsage[] }>(
    `query($teamId: ID!, $since: Time) { teamUsageByModel(teamId: $teamId, since: $since) { provider model ${USAGE_SUMMARY_FIELDS} } }`,
    { teamId, since: window.since }
  );
  return data.teamUsageByModel;
}

export async function getTeamUsageDaily(teamId: string, window: UsageWindow = {}): Promise<DailyUsage[]> {
  const data = await gql<{ teamUsageDaily: DailyUsage[] }>(
    `query($teamId: ID!, $since: Time, $until: Time) { teamUsageDaily(teamId: $teamId, since: $since, until: $until) { date requestCount totalTokens costUsd } }`,
    { teamId, since: window.since, until: window.until }
  );
  return data.teamUsageDaily;
}

export async function getGlobalUsage(window: UsageWindow = {}): Promise<UsageSummary> {
  const data = await gql<{ globalUsage: UsageSummary }>(
    `query($since: Time) { globalUsage(since: $since) { ${USAGE_SUMMARY_FIELDS} } }`,
    { since: window.since }
  );
  return data.globalUsage;
}

export async function getGlobalUsageByTeam(window: UsageWindow = {}): Promise<TeamUsage[]> {
  const data = await gql<{ globalUsageByTeam: TeamUsage[] }>(
    `query($since: Time) { globalUsageByTeam(since: $since) { teamId name ${USAGE_SUMMARY_FIELDS} } }`,
    { since: window.since }
  );
  return data.globalUsageByTeam;
}
