import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";

import { useAuth } from "../lib/auth";
import { Button } from "../components/ui/Button";
import { ErrorState } from "../components/ui/States";

export function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await login(username, password);
      navigate("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex h-full items-center justify-center">
      <form onSubmit={onSubmit} className="w-80 rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] p-6">
        <div className="mb-1 text-[15px] font-bold tracking-widest text-[var(--color-text)]">ULPF</div>
        <div className="mb-5 text-[11px] text-[var(--color-text-muted)]">Universal Log Pre-processing Framework</div>

        <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">Username</label>
        <input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="mb-3 w-full rounded border border-[var(--color-border-strong)] bg-black/30 px-2.5 py-1.5 text-[13px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
          autoComplete="username"
        />

        <label className="mb-1 block text-[11px] text-[var(--color-text-secondary)]">Password</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="mb-4 w-full rounded border border-[var(--color-border-strong)] bg-black/30 px-2.5 py-1.5 text-[13px] text-[var(--color-text)] outline-none focus:border-[var(--color-accent)]"
          autoComplete="current-password"
        />

        {error && (
          <div className="mb-3">
            <ErrorState message={error} />
          </div>
        )}

        <Button type="submit" variant="primary" disabled={submitting} className="w-full justify-center">
          {submitting ? "Signing in…" : "Sign in"}
        </Button>

        <div className="mt-4 text-[10.5px] text-[var(--color-text-muted)]">
          Dev default: admin / admin (set ULPF_ADMIN_PASSWORD to change).
        </div>
      </form>
    </div>
  );
}
