"use client";

import { useCallback, useRef } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";

/**
 * Tap handling for the hot path.
 *
 * `pointerdown` fires when the finger lands; `click` only fires when it
 * lifts. On the add-expense flow — seven taps in a row — using pointerdown is
 * worth having, so this keeps it.
 *
 * But pointerdown alone is a bug: pressing Enter or Space on a focused button
 * fires `click` and nothing else, so a pointerdown-only button is completely
 * unusable from a keyboard. This wires both and makes sure the handler runs
 * exactly once per activation.
 */
export function useTap() {
  const handledByPointer = useRef(false);

  return useCallback((handler: () => void) => {
    const markHandled = () => {
      handledByPointer.current = true;
      // Self-healing: if the matching click never arrives (the finger slid
      // off, or the gesture was cancelled), stop swallowing later clicks.
      setTimeout(() => {
        handledByPointer.current = false;
      }, 400);
    };

    return {
      onPointerDown: (e: ReactPointerEvent<HTMLButtonElement>) => {
        if (e.button !== 0) return;
        // Keeps the press from stealing focus or starting a text selection.
        e.preventDefault();
        markHandled();
        handler();
      },
      onClick: () => {
        // Reached by keyboard activation, assistive tech, and any environment
        // that synthesises a click without a pointer sequence.
        if (handledByPointer.current) {
          handledByPointer.current = false;
          return;
        }
        handler();
      },
    };
  }, []);
}
