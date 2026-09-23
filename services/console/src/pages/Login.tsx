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
    <div className="relative flex h-full min-h-[620px] overflow-auto bg-[var(--color-bg)]">
      <div className="pointer-events-none absolute inset-0 opacity-25" style={{ backgroundImage: "linear-gradient(rgba(207,219,244,.025) 1px,transparent 1px),linear-gradient(90deg,rgba(207,219,244,.025) 1px,transparent 1px)", backgroundSize: "56px 56px" }} />
      <div className="pointer-events-none absolute left-[16%] top-[18%] h-[420px] w-[420px] rounded-full bg-[var(--color-accent)]/[0.13] blur-[140px]" />
      <div className="pointer-events-none absolute bottom-[-10%] right-[8%] h-[360px] w-[360px] rounded-full bg-[var(--color-blue)]/[0.11] blur-[140px]" />

      <section className="relative hidden w-[55%] flex-col justify-between border-r border-white/[0.06] p-12 lg:flex xl:p-16">
        <div className="flex items-center gap-3">
          <div className="neo-button grid h-11 w-11 place-items-center rounded-xl border border-[var(--color-accent)]/35 bg-[var(--color-accent)]/10">
            <ShieldIcon />
          </div>
          <div><div className="text-[19px] font-semibold tracking-[0.08em] text-white">LogKrama</div><div className="mt-0.5 text-[9px] uppercase tracking-[0.14em] text-[var(--color-text-muted)]">Security intelligence workspace</div></div>
        </div>

        <div className="max-w-[670px]">
          <div className="eyebrow text-[#9e82ef]">Security operations platform</div>
          <h1 className="mt-5 max-w-[620px] text-[44px] font-light leading-[1.08] tracking-[-0.045em] text-white xl:text-[56px]">Every event.<br /><span className="bg-[linear-gradient(95deg,#b59cff,#5e95ff)] bg-clip-text font-medium text-transparent">One secure language.</span></h1>
          <p className="mt-6 max-w-[560px] text-[16px] leading-8 text-[var(--color-text-secondary)]">Bring fragmented telemetry into one clear, trustworthy view — with evidence that remains traceable from source to decision.</p>
          <div className="neo-card mt-10 grid max-w-[580px] grid-cols-3 divide-x divide-white/[0.07] rounded-2xl border border-white/[0.07] py-5">
            <LoginStat value="08" label="Processing stages" />
            <LoginStat value="SHA-256" label="Evidence integrity" />
            <LoginStat value="AIR-GAP" label="Ready architecture" />
          </div>
        </div>

        <div className="flex items-center gap-4 text-[8.5px] uppercase tracking-[0.14em] text-[var(--color-text-muted)]"><span>Protected environment</span><span className="h-px w-8 bg-white/[0.15]" /><span>Authorized access only</span></div>
      </section>

      <section className="relative flex min-h-full flex-1 items-center justify-center p-6 sm:p-10">
        <form onSubmit={onSubmit} className="security-panel w-full max-w-[440px] rounded-3xl border border-white/[0.08] bg-[linear-gradient(145deg,#1b2432,#111822)] p-8 backdrop-blur-xl sm:p-10">
          <div className="mb-8 lg:hidden"><div className="text-[16px] font-semibold tracking-[0.2em]">LogKrama</div><div className="mt-1 text-[9px] uppercase tracking-[0.15em] text-[var(--color-text-muted)]">Security operations platform</div></div>
          <div className="eyebrow">Identity gateway</div>
          <h2 className="mt-3 text-[30px] font-medium tracking-[-0.03em] text-white">Welcome back</h2>
          <p className="mt-2 text-[14px] leading-relaxed text-[var(--color-text-muted)]">Sign in to your LogKrama workspace.</p>

          <div className="mt-8">
            <label className="mb-2 block text-[9px] font-semibold uppercase tracking-[0.13em] text-[var(--color-text-secondary)]">Operator identity</label>
            <div className="relative">
              <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)]"><UserIcon /></span>
              <input value={username} onChange={(e) => setUsername(e.target.value)} className="h-12 w-full rounded-xl border pl-10 pr-3 text-[14px] text-white outline-none" autoComplete="username" />
            </div>
          </div>
          <div className="mt-4">
            <div className="mb-2 flex items-center justify-between"><label className="text-[9px] font-semibold uppercase tracking-[0.13em] text-[var(--color-text-secondary)]">Secure credential</label><span className="font-mono-data text-[8px] text-[var(--color-text-muted)]">AES / TLS</span></div>
            <div className="relative">
              <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)]"><LockIcon /></span>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="h-12 w-full rounded-xl border pl-10 pr-3 text-[14px] text-white outline-none" autoComplete="current-password" placeholder="Enter password" />
            </div>
          </div>

          {error && <div className="mt-4"><ErrorState message={error} /></div>}
          <Button type="submit" variant="primary" disabled={submitting} className="mt-6 h-11 w-full">
            {submitting ? "Authenticating…" : "Authenticate & enter"}<span aria-hidden className="ml-auto">→</span>
          </Button>

          <div className="mt-6 flex items-center justify-between border-t border-white/[0.08] pt-5">
            <div className="flex items-center gap-2"><span className="status-pulse h-1.5 w-1.5 rounded-full bg-[var(--color-success)]" /><span className="text-[9px] text-[var(--color-text-muted)]">Identity service online</span></div>
            <span className="font-mono-data text-[8px] text-[var(--color-text-muted)]">admin / admin</span>
          </div>
        </form>
      </section>
    </div>
  );
}

function LoginStat({ value, label }: { value: string; label: string }) {
  return <div className="px-6 first:pl-0"><div className="font-mono-data text-[16px] font-medium text-white">{value}</div><div className="mt-1.5 text-[8px] uppercase tracking-[0.13em] text-[var(--color-text-muted)]">{label}</div></div>;
}
function ShieldIcon() { return <svg viewBox="0 0 24 24" className="h-5 w-5 text-[#b29aff]" fill="none" stroke="currentColor" strokeWidth="1.4"><path d="M12 2.8 20 6v5.6c0 4.7-3.2 8.1-8 9.6-4.8-1.5-8-4.9-8-9.6V6l8-3.2Z" /><path d="m8.5 12 2.1 2.1 5-5" /></svg>; }
function UserIcon() { return <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.5"><circle cx="12" cy="8" r="3.5" /><path d="M5 20c.6-4 3-6 7-6s6.4 2 7 6" /></svg>; }
function LockIcon() { return <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="5" y="10" width="14" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v2" /></svg>; }
