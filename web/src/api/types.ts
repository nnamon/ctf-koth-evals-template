// Wire types — keep in sync with internal/wireapi/*.go on the Go side.

export type Challenge = {
  id: number;
  name: string;
  version: string;
  description?: string;
  bundle_size: number;
  created_at: string;
};

export type Suite = {
  id: number;
  name: string;
  description?: string;
  challenge: Challenge;
  parameters?: Record<string, unknown>;
  seeds: string[];
  timeout_seconds: number;
  scoring?: Record<string, unknown>;
  sealed: boolean;
  created_at: string;
};

export type CreateSuiteRequest = {
  name: string;
  description?: string;
  challenge_id: number;
  parameters?: Record<string, unknown>;
  seeds: string[];
  timeout_seconds?: number;
  scoring?: Record<string, unknown>;
};

type SubmissionBase = {
  id: number;
  name?: string;
  submitter?: string;
  artifact_name: string;
  artifact_size: number;
  created_at: string;
};

export type SubmissionSummary = SubmissionBase & {
  runs: RunCounts;
};

export type RunStatus =
  | "pending"
  | "claimed"
  | "running"
  | "succeeded"
  | "failed"
  | "timed_out"
  | "cancelled";

export type RunSummary = {
  id: number;
  suite_id: number;
  seed: string;
  status: RunStatus;
  score?: number;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
};

export type SubmissionDetail = SubmissionBase & {
  runs: RunSummary[];
};

export type RunDetail = RunSummary & {
  submission_id: number;
  submission_name?: string;
  result?: Record<string, unknown>;
  worker_id?: string;
  claimed_at?: string;
  stdout?: string;
  stderr?: string;
};

export type RunCounts = {
  total: number;
  succeeded: number;
  failed: number;
  timed_out: number;
  pending: number;
  cancelled: number;
};

export type Metrics = {
  mean: number;
  median: number;
  mode: number;
  max: number;
  min: number;
  stddev: number;
};

export type QueueSuiteEntry = {
  suite_id: number;
  name: string;
  pending: number;
  mean_duration_ms: number;
};

export type QueueStatus = {
  pending_total: number;
  active_workers: number;
  throughput_per_sec: number;
  eta_seconds?: number;
  by_suite: QueueSuiteEntry[];
};

export type SubmissionDistribution = {
  submission_id: number;
  name?: string;
  submitter?: string;
  scores: number[];
  submitted_at: string;
};

export type LeaderboardEntry = {
  submission_id: number;
  name?: string;
  submitter?: string;
  metrics?: Metrics;
  /** Half-width of the 95% CI around the mean (mean ± this value). 0 when n<2. */
  ci95_half_width: number;
  /** Two-tailed paired t-test p-value vs the top-mean entry. null on the leader. */
  p_value_vs_leader?: number;
  runs: RunCounts;
  submitted_at: string;
};
