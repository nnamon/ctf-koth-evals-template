import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { api } from "../api/client";
import type { SubmissionSummary } from "../api/types";

export function Submissions() {
  const [items, setItems] = useState<SubmissionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .listSubmissions()
      .then((rows) => !cancelled && setItems(rows))
      .catch((err) => !cancelled && setError(err.message ?? String(err)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <span className="current">submissions</span>
      </nav>
      <main>
          <h1>Submissions</h1>
          {error && <Alert title="Error">{error}</Alert>}
          {loading ? (
            <p className="t-cmt">loading…</p>
          ) : items.length === 0 ? (
            <p className="t-cmt">none yet</p>
          ) : (
            <div className="data-scroll">
              <table className="data" style={{ width: "100%" }}>
                <thead>
                  <tr>
                    <th style={{ textAlign: "left" }}>#</th>
                    <th style={{ textAlign: "left" }}>Name</th>
                    <th style={{ textAlign: "left" }}>Submitter</th>
                    <th style={{ textAlign: "left" }}>Artifact</th>
                    <th style={{ textAlign: "right" }}>Size</th>
                    <th style={{ textAlign: "left" }}>Submitted</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((s) => (
                    <tr key={s.id}>
                      <td>
                        <code className="t-num">{s.id}</code>
                      </td>
                      <td>
                        <Link to={`/submissions/${s.id}`}>
                          {s.name || <code className="t-cmt">unnamed</code>}
                        </Link>
                      </td>
                      <td>
                        {s.submitter ? (
                          <code className="t-cmt">{s.submitter}</code>
                        ) : (
                          <code className="t-cmt">—</code>
                        )}
                      </td>
                      <td>
                        <code className="t-path">{s.artifact_name}</code>
                      </td>
                      <td style={{ textAlign: "right" }}>
                        <code className="t-num">{s.artifact_size}</code>
                      </td>
                      <td>
                        <code className="t-num">
                          {new Date(s.created_at).toLocaleString()}
                        </code>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
      </main>
    </div>
  );
}
