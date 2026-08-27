"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

/**
 * Toasts exist here for one reason: a mutation that fails must be loud.
 *
 * There is deliberately no success toast for a save. The optimistic row
 * turning solid in the expenses list is the success signal, and a toast on
 * top of that would be two signals for one event on the app's most frequent
 * action. Failure is the case that needs words and a button.
 */

export type ToastAction = { label: string; onClick: () => void };

export type Toast = {
  id: string;
  kind: "error" | "info";
  message: string;
  detail?: string;
  action?: ToastAction;
};

type ToastInput = Omit<Toast, "id">;

type ToastContextValue = {
  show: (t: ToastInput) => string;
  dismiss: (id: string) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const AUTO_DISMISS_MS = 2_600;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const seq = useRef(0);

  const dismiss = useCallback((id: string) => {
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const show = useCallback(
    (t: ToastInput) => {
      const id = `t${++seq.current}`;
      setToasts((list) => [...list, { ...t, id }]);
      // Errors stay until the user acts on them. Losing a failure notice to a
      // timer is exactly how a save silently fails.
      if (t.kind !== "error") {
        setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
      }
      return id;
    },
    [dismiss],
  );

  const value = useMemo(() => ({ show, dismiss }), [show, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <ToastHost toasts={toasts} dismiss={dismiss} />
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside <ToastProvider>");
  return ctx;
}

function ToastHost({
  toasts,
  dismiss,
}: {
  toasts: Toast[];
  dismiss: (id: string) => void;
}) {
  if (toasts.length === 0) return null;

  return (
    <div
      // Sits above the tab bar so it is never hidden behind it.
      className="pointer-events-none fixed inset-x-0 bottom-0 z-50 flex flex-col gap-2 px-3 pb-[calc(env(safe-area-inset-bottom,0px)+5.75rem)]"
      role="region"
      aria-label="Notifications"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          role={t.kind === "error" ? "alert" : "status"}
          aria-live={t.kind === "error" ? "assertive" : "polite"}
          className={`pointer-events-auto flex items-start gap-3 rounded-xl border px-4 py-3 shadow-lg ${
            t.kind === "error"
              ? "border-brick/30 bg-brick-tint text-ink"
              : "border-line bg-paper-raised text-ink"
          }`}
        >
          <div className="min-w-0 flex-1">
            <p
              className={`text-sm font-medium ${
                t.kind === "error" ? "text-brick" : "text-ink"
              }`}
            >
              {t.message}
            </p>
            {t.detail ? (
              <p className="mt-0.5 truncate text-xs text-ink-soft">
                {t.detail}
              </p>
            ) : null}
          </div>

          {t.action ? (
            <button
              type="button"
              onClick={() => {
                dismiss(t.id);
                t.action?.onClick();
              }}
              className="min-h-11 shrink-0 rounded-lg bg-brick px-3.5 text-sm font-semibold text-white"
            >
              {t.action.label}
            </button>
          ) : null}

          <button
            type="button"
            onClick={() => dismiss(t.id)}
            aria-label="Dismiss"
            className="-mr-1 min-h-11 shrink-0 px-2 text-lg leading-none text-ink-soft"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
