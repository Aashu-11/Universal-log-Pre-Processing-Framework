import { type ReactNode } from "react";
import { NavLink } from "react-router-dom";

import { useAuth } from "../../lib/auth";
import { Badge } from "../ui/Badge";

const NAV_ITEMS = [
  { to: "/", label: "Live Theater", end: true },
  { to: "/pipeline", label: "Pipeline" },
  { to: "/sources", label: "Sources" },
  { to: "/explorer", label: "Explorer" },
  { to: "/traceability", label: "Traceability" },
  { to: "/workbench", label: "Parser Workbench" },
  { to: "/dlq", label: "DLQ" },
  { to: "/reviewer", label: "Reviewer Mode" },
];

export function AppShell({ children }: { children: ReactNode }) {
  const { role, username, logout } = useAuth();

  return (
    <div className="flex h-full">
      <aside className="flex w-56 shrink-0 flex-col border-r border-[var(--color-border)] bg-[var(--color-surface)]">
        <div className="border-b border-[var(--color-border)] px-4 py-3.5">
          <div className="text-[13px] font-bold tracking-widest text-[var(--color-text)]">ULPF</div>
          <div className="text-[10px] text-[var(--color-text-muted)]">Universal Log Pre-processing</div>
        </div>
        <nav className="flex-1 space-y-0.5 p-2">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `block rounded px-3 py-1.5 text-[12.5px] font-medium transition-colors ${
                  isActive
                    ? "bg-[var(--color-accent)]/15 text-[var(--color-accent)]"
                    : "text-[var(--color-text-secondary)] hover:bg-white/5 hover:text-[var(--color-text)]"
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="border-t border-[var(--color-border)] p-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-[12px] text-[var(--color-text)]">{username}</span>
            {role && <Badge tone="accent">{role}</Badge>}
          </div>
          <button
            onClick={logout}
            className="w-full rounded border border-[var(--color-border-strong)] py-1 text-[11.5px] text-[var(--color-text-secondary)] hover:bg-white/5"
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-[1400px] p-6">{children}</div>
      </main>
    </div>
  );
}
