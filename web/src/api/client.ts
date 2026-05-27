import type {
  Challenge,
  CreateSuiteRequest,
  LeaderboardEntry,
  QueueStatus,
  RunDetail,
  Suite,
  SubmissionDetail,
  SubmissionSummary,
} from "./types";

export class ApiError extends Error {
  status: number;
  body: string;
  constructor(status: number, body: string) {
    super(`API ${status}: ${body || "(empty body)"}`);
    this.status = status;
    this.body = body;
  }
}

// onUnauthorized is invoked when any request returns 401. Used by the auth
// layer to surface "your session is invalid, go back to /login".
let onUnauthorized: (() => void) | null = null;
export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn;
}

function authHeader(): string | null {
  const creds = sessionStorage.getItem("ctf-evals.creds");
  return creds ? `Basic ${creds}` : null;
}

async function request<T>(
  path: string,
  init: RequestInit & { json?: unknown } = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  const auth = authHeader();
  if (auth) headers.set("Authorization", auth);

  let body = init.body;
  if (init.json !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(init.json);
  }

  const res = await fetch(path, { ...init, headers, body });
  if (res.status === 401 && onUnauthorized) onUnauthorized();
  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  me: () => request<{ authenticated: boolean }>("/api/me"),
  listChallenges: () => request<Challenge[]>("/api/challenges"),
  listSuites: () => request<Suite[]>("/api/suites"),
  getSuite: (id: number) => request<Suite>(`/api/suites/${id}`),
  createSuite: (body: CreateSuiteRequest) =>
    request<Suite>("/api/suites", { method: "POST", json: body }),
  leaderboard: (id: number) =>
    request<LeaderboardEntry[]>(`/api/suites/${id}/leaderboard`),
  listSubmissions: () => request<SubmissionSummary[]>("/api/submissions"),
  getSubmission: (id: number) =>
    request<SubmissionDetail>(`/api/submissions/${id}`),
  getRun: (id: number) => request<RunDetail>(`/api/runs/${id}`),
  queue: () => request<QueueStatus>("/api/queue"),

  cancelSubmission: (id: number) =>
    request<{ cancelled: number }>(`/api/submissions/${id}/cancel`, { method: "POST" }),
  retrySubmission: (id: number) =>
    request<{ retried: number }>(`/api/submissions/${id}/retry`, { method: "POST" }),
  prioritizeSubmission: (id: number) =>
    request<{ prioritized: number }>(`/api/submissions/${id}/prioritize`, { method: "POST" }),
  uploadSubmission: async (
    suiteIds: number[],
    name: string,
    submitter: string,
    file: File,
  ): Promise<SubmissionDetail> => {
    const form = new FormData();
    for (const id of suiteIds) form.append("suite_ids", String(id));
    if (name) form.set("name", name);
    if (submitter) form.set("submitter", submitter);
    form.set("artifact", file);
    return request<SubmissionDetail>("/api/submissions", {
      method: "POST",
      body: form,
    });
  },
};
