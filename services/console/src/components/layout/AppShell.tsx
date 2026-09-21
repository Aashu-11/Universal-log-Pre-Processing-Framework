import { useEffect, useState, type ReactNode } from "react";
import { NavLink } from "react-router-dom";

import { useAuth } from "../../lib/auth";
import { Badge } from "../ui/Badge";

type IconName = "activity" | "pipeline" | "sources" | "search" | "trace" | "code" | "alert" | "shield";

const NAV_ITEMS: { to: string; label: string; description: string; icon: IconName; end?: boolean }[] = [
  { to: "/", label: "Live Theater", description: "Event stream", icon: "activity", end: true },
  { to: "/pipeline", label: "Pipeline", description: "System health", icon: "pipeline" },
  { to: "/sources", label: "Sources", description: "Asset inventory", icon: "sources" },
  { to: "/explorer", label: "Explorer", description: "Federated search", icon: "search" },
  { to: "/traceability", label: "Traceability", description: "Chain of custody", icon: "trace" },
  { to: "/workbench", label: "Parser Workbench", description: "Detection engineering", icon: "code" },
  { to: "/dlq", label: "Dead Letter Queue", description: "Exception review", icon: "alert" },
  { to: "/reviewer", label: "Reviewer Mode", description: "Control validation", icon: "shield" },
];

export function AppShell({ children }: { children: ReactNode }) {
  const { role, username, logout } = useAuth();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <div className="flex h-full min-w-[320px] bg-transparent">
      <aside className="hidden w-[264px] shrink-0 flex-col border-r border-white/[0.08] bg-[#070709]/95 lg:flex">
        <div className="flex h-[76px] items-center border-b border-white/[0.08] px-5">
          <BrandMark />
          <div className="ml-3 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-[15px] font-semibold tracking-[0.18em] text-white">ULPF</span>
              <span className="rounded border border-[var(--color-accent)]/25 bg-[var(--color-accent)]/10 px-1.5 py-0.5 font-mono-data text-[7px] font-semibold tracking-wider text-[#b9a2ff]">SOC</span>
            </div>
            <div className="mt-0.5 truncate text-[9px] uppercase tracking-[0.16em] text-[var(--color-text-muted)]">Universal telemetry fabric</div>
          </div>
        </div>

        <div className="px-4 pb-2 pt-5"><div className="eyebrow px-2">Security operations</div></div>
        <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-2">
          {NAV_ITEMS.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) =>
              `group relative flex items-center gap-3 rounded-lg border px-3 py-2.5 transition-all duration-200 ${isActive ? "border-[var(--color-accent)]/25 bg-[linear-gradient(100deg,rgba(124,79,224,0.17),rgba(47,92,190,0.07))] text-white" : "border-transparent text-[var(--color-text-secondary)] hover:border-white/[0.07] hover:bg-white/[0.025] hover:text-white"}`
            }>
              {({ isActive }) => <>
                {isActive && <span className="absolute -left-[13px] h-7 w-[2px] rounded-r bg-[var(--color-accent)] shadow-[0_0_14px_var(--color-accent)]" />}
                <span className={`grid h-8 w-8 shrink-0 place-items-center rounded-md border ${isActive ? "border-[var(--color-accent)]/25 bg-[var(--color-accent)]/12 text-[#aa8dfd]" : "border-white/[0.07] bg-white/[0.025] text-[var(--color-text-muted)] group-hover:text-[var(--color-text-secondary)]"}`}><NavIcon name={item.icon} /></span>
                <span className="min-w-0">
                  <span className="block truncate text-[11.5px] font-medium">{item.label}</span>
                  <span className="mt-0.5 block truncate text-[8.5px] uppercase tracking-[0.12em] text-[var(--color-text-muted)]">{item.description}</span>
                </span>
              </>}
            </NavLink>
          ))}
        </nav>

        <div className="m-3 rounded-xl border border-white/[0.08] bg-white/[0.025] p-3.5">
          <div className="mb-3 flex items-center justify-between">
            <div className="flex items-center gap-2"><span className="status-pulse h-1.5 w-1.5 rounded-full bg-[var(--color-success)]" /><span className="text-[10px] font-medium text-[var(--color-text-secondary)]">System protected</span></div>
            <span className="font-mono-data text-[8px] text-[var(--color-text-muted)]">v1.0</span>
          </div>
          <div className="flex items-center justify-between border-t border-white/[0.07] pt-3">
            <div className="min-w-0"><div className="truncate text-[11px] font-medium text-white">{username}</div><div className="mt-0.5 text-[8.5px] uppercase tracking-wider text-[var(--color-text-muted)]">Authenticated session</div></div>
            {role && <Badge tone="accent">{role}</Badge>}
          </div>
          <button onClick={logout} className="mt-3 w-full rounded-md border border-white/[0.09] bg-black/20 py-1.5 text-[9px] font-semibold uppercase tracking-[0.13em] text-[var(--color-text-muted)] transition hover:border-white/[0.18] hover:text-white">Terminate session</button>
        </div>
      </aside>

      <section className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-[76px] shrink-0 items-center justify-between border-b border-white/[0.08] bg-[#070709]/75 px-4 backdrop-blur-xl sm:px-6">
          <div className="flex items-center gap-3 lg:hidden"><BrandMark /><span className="text-[13px] font-semibold tracking-[0.16em]">ULPF</span></div>
          <div className="hidden items-center gap-6 lg:flex">
            <div><div className="eyebrow">Unified security control plane</div><div className="mt-1 text-[11.5px] text-[var(--color-text-secondary)]">Enterprise telemetry / Primary environment</div></div>
          </div>
          <div className="flex items-center gap-3 sm:gap-5">
            <div className="hidden items-center gap-2 rounded-lg border border-white/[0.08] bg-black/20 px-3 py-2 sm:flex"><span className="status-pulse h-1.5 w-1.5 rounded-full bg-[var(--color-success)]" /><span className="text-[9px] font-semibold uppercase tracking-[0.13em] text-[var(--color-success)]">Operational</span></div>
            <div className="h-7 w-px bg-white/[0.09]" />
            <div className="text-right"><div className="font-mono-data text-[11px] text-white">{now.toLocaleTimeString([], { hour12: false })}</div><div className="mt-0.5 font-mono-data text-[8px] uppercase tracking-[0.1em] text-[var(--color-text-muted)]">{now.toLocaleDateString([], { day: "2-digit", month: "short", year: "numeric" })} / Local</div></div>
          </div>
        </header>

        <div className="border-b border-white/[0.08] bg-[#08080b]/95 px-3 py-2 lg:hidden">
          <nav className="flex gap-1 overflow-x-auto">
            {NAV_ITEMS.map((item) => <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => `shrink-0 rounded-md border px-3 py-1.5 text-[10px] font-medium ${isActive ? "border-[var(--color-accent)]/35 bg-[var(--color-accent)]/15 text-white" : "border-transparent text-[var(--color-text-muted)]"}`}>{item.label}</NavLink>)}
          </nav>
        </div>

        <main className="min-h-0 flex-1 overflow-auto"><div className="mx-auto w-full max-w-[1680px] p-4 sm:p-6 xl:p-8">{children}</div></main>
      </section>
    </div>
  );
}

function BrandMark() {
  return <div className="relative grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-[var(--color-accent)]/35 bg-[linear-gradient(145deg,rgba(139,92,246,0.22),rgba(59,130,246,0.08))] shadow-[0_0_24px_rgba(116,72,220,0.14)]">
    <svg viewBox="0 0 24 24" className="h-5 w-5 text-[#ae94ff]" fill="none" stroke="currentColor" strokeWidth="1.4"><path d="M12 2.8 20 6v5.6c0 4.7-3.2 8.1-8 9.6-4.8-1.5-8-4.9-8-9.6V6l8-3.2Z" /><path d="m8.4 12 2.1 2.1 5.1-5.2" /></svg>
    <span className="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-full border-2 border-[#08080a] bg-[var(--color-success)]" />
  </div>;
}

function NavIcon({ name }: { name: IconName }) {
  const paths: Record<IconName, ReactNode> = {
    activity: <><path d="M3 12h3l2-5 4 10 2-5h7" /><path d="M5 4h14a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z" /></>,
    pipeline: <><rect x="3" y="4" width="5" height="5" rx="1" /><rect x="16" y="4" width="5" height="5" rx="1" /><rect x="9.5" y="15" width="5" height="5" rx="1" /><path d="M8 6.5h8M18.5 9v2.5a2 2 0 0 1-2 2h-9a2 2 0 0 0-2 2V17h4" /></>,
    sources: <><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6" /></>,
    search: <><circle cx="10.5" cy="10.5" r="6.5" /><path d="m15.5 15.5 5 5M8 8h5M8 11h4" /></>,
    trace: <><path d="M5 4v12a4 4 0 0 0 4 4h10" /><circle cx="5" cy="4" r="2" /><circle cx="12" cy="10" r="2" /><circle cx="19" cy="20" r="2" /><path d="M7 10h3M12 12v3a3 3 0 0 0 3 3h2" /></>,
    code: <><path d="m8 8-4 4 4 4M16 8l4 4-4 4M14 4l-4 16" /></>,
    alert: <><path d="M12 3 2.8 19h18.4L12 3Z" /><path d="M12 9v4M12 16.5v.1" /></>,
    shield: <><path d="M12 2.8 20 6v5.6c0 4.7-3.2 8.1-8 9.6-4.8-1.5-8-4.9-8-9.6V6l8-3.2Z" /><path d="m8.5 12 2.2 2.2 4.9-5" /></>,
  };
  return <svg viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}
