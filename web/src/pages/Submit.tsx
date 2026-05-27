import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";
import { api } from "../api/client";
import type { Suite } from "../api/types";

export function Submit() {
  const navigate = useNavigate();
  const location = useLocation();
  const presetSuite = new URLSearchParams(location.search).get("suite");

  const [suites, setSuites] = useState<Suite[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [name, setName] = useState("");
  const [submitter, setSubmitter] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .listSuites()
      .then((rows) => {
        if (cancelled) return;
        setSuites(rows);
        if (presetSuite) {
          const id = Number(presetSuite);
          if (rows.some((s) => s.id === id)) {
            setSelected(new Set([id]));
          }
        }
      })
      .catch((err) => !cancelled && setError(err.message ?? String(err)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [presetSuite]);

  const toggle = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!file) {
      setError("Pick a file to upload.");
      return;
    }
    if (selected.size === 0) {
      setError("Select at least one suite.");
      return;
    }
    setSubmitting(true);
    try {
      const sub = await api.uploadSubmission(
        Array.from(selected),
        name,
        submitter,
        file,
      );
      navigate(`/submissions/${sub.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="page">
      <PageHeader />
      <nav className="breadcrumb">
        <Link to="/">home</Link>
        <span className="sep">/</span>
        <span className="current">submit</span>
      </nav>
      <main>
          <h1>Submit</h1>
          <p>
            One artifact, one or more suites. The platform fans out runs across{" "}
            <code className="t-kw">suite × seed</code> for every selected suite.
          </p>

          {error && <Alert title="Couldn't submit">{error}</Alert>}

          {loading ? (
            <p className="t-cmt">loading suites…</p>
          ) : suites.length === 0 ? (
            <Alert title="No suites yet">
              <Link to="/suites/new">Create a suite</Link> first.
            </Alert>
          ) : (
            <form
              onSubmit={onSubmit}
              style={{ display: "grid", gap: "var(--space-4)", maxWidth: 720 }}
            >
              <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
                <label className="field-label">
                  Name
                  <span className="hint">
                    Label for this submission (e.g. <code className="t-path">v3-greedy</code>). Defaults to the artifact's filename.
                  </span>
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="v1"
                />
              </div>

              <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
                <label className="field-label">
                  Submitter
                  <span className="hint">Optional. Who submitted this — left blank if it's just you.</span>
                </label>
                <input
                  type="text"
                  value={submitter}
                  onChange={(e) => setSubmitter(e.target.value)}
                />
              </div>

              <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
                <label className="field-label">Artifact</label>
                <input
                  type="file"
                  onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                  required
                />
              </div>

              <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
                <label className="field-label">
                  Suites
                  <span className="hint">
                    Pick one or more. Each will receive its own set of runs.
                  </span>
                </label>
                <div className="data-scroll">
                  <table className="data" style={{ width: "100%" }}>
                    <thead>
                      <tr>
                        <th style={{ width: "32px" }}></th>
                        <th style={{ textAlign: "left" }}>Suite</th>
                        <th style={{ textAlign: "left" }}>Challenge</th>
                        <th style={{ textAlign: "right" }}>Seeds</th>
                        <th style={{ textAlign: "left" }}>Sealed</th>
                      </tr>
                    </thead>
                    <tbody>
                      {suites.map((s) => (
                        <tr key={s.id}>
                          <td>
                            <input
                              id={`suite-${s.id}`}
                              type="checkbox"
                              checked={selected.has(s.id)}
                              onChange={() => toggle(s.id)}
                            />
                          </td>
                          <td>
                            <label htmlFor={`suite-${s.id}`} style={{ cursor: "pointer" }}>
                              {s.name}
                            </label>
                          </td>
                          <td>
                            <code className="t-type">{s.challenge.name}</code>
                          </td>
                          <td style={{ textAlign: "right" }}>
                            <code className="t-num">{s.seeds.length}</code>
                          </td>
                          <td>
                            {s.sealed ? (
                              <code className="t-str">yes</code>
                            ) : (
                              <code className="t-kw">no</code>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <p className="t-cmt" style={{ fontFamily: "var(--mono)", fontSize: "12px" }}>
                  {selected.size} selected ·{" "}
                  {Array.from(selected)
                    .map((id) => suites.find((s) => s.id === id)?.seeds.length ?? 0)
                    .reduce((a, b) => a + b, 0)}{" "}
                  runs will be created
                </p>
              </div>

              <div>
                <button type="submit" className="btn" disabled={submitting}>
                  {submitting ? "Submitting…" : "Submit"}
                </button>
              </div>
            </form>
          )}
      </main>
    </div>
  );
}
