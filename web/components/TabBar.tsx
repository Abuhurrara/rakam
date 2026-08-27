"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { countTap, mark, resetAddFlow } from "@/lib/perf";
import { useTap } from "@/lib/useTap";
import { useAddSheet } from "./AddSheet";

/**
 * Fixed bottom tab bar with the floating "+" above it, on every tab.
 *
 * Home, Ledger, Budget and More are honest stubs in this phase — the bar is
 * real, its destinations arrive in phase 7.
 */

const TABS = [
  { href: "/", label: "Home", icon: HomeIcon },
  { href: "/expenses", label: "Expenses", icon: ListIcon },
  { href: "/ledger", label: "Ledger", icon: PeopleIcon },
  { href: "/budget", label: "Budget", icon: GaugeIcon },
  { href: "/more", label: "More", icon: MoreIcon },
] as const;

export function TabBar() {
  const pathname = usePathname();
  const { open } = useAddSheet();
  const tap = useTap();

  return (
    <>
      <button
        type="button"
        aria-label="Add expense"
        {...tap(() => {
          // Reset here, not when the sheet mounts — the sheet mounts *after*
          // this tap, so resetting there would erase the fabTap mark and the
          // tap that opened the flow.
          resetAddFlow();
          countTap();
          mark("fabTap");
          open();
        })}
        className="fixed bottom-[calc(env(safe-area-inset-bottom,0px)+4.75rem)] right-4 z-30 flex h-14 w-14 items-center justify-center rounded-full bg-primary text-primary-ink shadow-lg active:scale-95"
      >
        <svg width="26" height="26" viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 5v14M5 12h14"
            stroke="currentColor"
            strokeWidth="2.2"
            strokeLinecap="round"
          />
        </svg>
      </button>

      <nav
        aria-label="Main"
        className="safe-bottom fixed inset-x-0 bottom-0 z-30 border-t border-line bg-paper-raised"
      >
        <ul className="mx-auto flex max-w-lg">
          {TABS.map((tab) => {
            const active =
              tab.href === "/"
                ? pathname === "/"
                : pathname.startsWith(tab.href);
            const Icon = tab.icon;
            return (
              <li key={tab.href} className="flex-1">
                <Link
                  href={tab.href}
                  aria-current={active ? "page" : undefined}
                  className={`flex min-h-[3.25rem] flex-col items-center justify-center gap-0.5 text-[0.6875rem] ${
                    active ? "font-medium text-primary" : "text-ink-faint"
                  }`}
                >
                  <Icon active={active} />
                  {tab.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </>
  );
}

type IconProps = { active: boolean };

function base(active: boolean) {
  return {
    width: 21,
    height: 21,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: active ? 2.1 : 1.7,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
}

function HomeIcon({ active }: IconProps) {
  return (
    <svg {...base(active)}>
      <path d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1h-4v-6H9v6H5a1 1 0 0 1-1-1v-9.5Z" />
    </svg>
  );
}

function ListIcon({ active }: IconProps) {
  return (
    <svg {...base(active)}>
      <path d="M4 7h16M4 12h16M4 17h10" />
    </svg>
  );
}

function PeopleIcon({ active }: IconProps) {
  return (
    <svg {...base(active)}>
      <circle cx="9" cy="8" r="3.2" />
      <path d="M3.5 19a5.5 5.5 0 0 1 11 0M16 11.2A3 3 0 0 0 16 5.4M18 19a5 5 0 0 0-2.6-4.4" />
    </svg>
  );
}

function GaugeIcon({ active }: IconProps) {
  return (
    <svg {...base(active)}>
      <path d="M4 18a8 8 0 1 1 16 0" />
      <path d="m12 18 4-5" />
    </svg>
  );
}

function MoreIcon({ active }: IconProps) {
  return (
    <svg {...base(active)}>
      <circle cx="5.5" cy="12" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="18.5" cy="12" r="1.4" fill="currentColor" stroke="none" />
    </svg>
  );
}
