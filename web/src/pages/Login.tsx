import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { PageHeader } from "../components/PageHeader";
import { Alert } from "../components/Alert";

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation() as { state?: { from?: string } };
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(password);
      navigate(location.state?.from ?? "/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="page narrow">
      <PageHeader />
      <main style={{ maxWidth: 360, margin: "var(--space-7) auto" }}>
          <h1 style={{ fontSize: "20px" }}>Sign in</h1>
          <p className="t-cmt" style={{ fontFamily: "var(--mono)" }}>
            // shared password gates /api/*
          </p>
          <form onSubmit={onSubmit} style={{ marginTop: "var(--space-5)" }}>
            <div className="field" style={{ display: "grid", gap: "var(--space-2)" }}>
              <label htmlFor="password" className="field-label">
                Password
              </label>
              <input
                id="password"
                type="password"
                autoFocus
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            {error && (
              <Alert title="Login failed" style={{ marginTop: "var(--space-4)" }}>
                {error}
              </Alert>
            )}
            <div style={{ marginTop: "var(--space-5)" }}>
              <button type="submit" className="btn" disabled={submitting}>
                {submitting ? "Signing in…" : "Sign in"}
              </button>
            </div>
          </form>
      </main>
    </div>
  );
}
