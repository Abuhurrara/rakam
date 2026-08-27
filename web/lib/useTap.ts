"use client";

import { useCallback, useRef } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";

/**
 * Tap handling, with one rule that matters on a touchscreen.
 *
 * On a phone, `pointerdown` fires when the finger lands and `click` fires
 * when it lifts — often 100-200ms later. If the handler makes the button
 * disappear in between, the browser still delivers that `click`, and it goes
 * to whatever is now underneath. That is a "ghost click".
 *
 * It bit exactly this app: Save sits on top of the tab bar, so saving an
 * expense fired on finger-down, the sheet closed, and the finger lifting
 * landed on the Budget tab. Perfect on a laptop, where down and click are
 * effectively simultaneous. Broken on every phone.
 *
 * Hence:
 *
 *   tap(fn)                  click only — for anything that closes, opens or
 *                            navigates, i.e. anything that changes what is
 *                            under the finger.
 *   tap(fn, { fast: true })  fires on finger-down — only for controls that
 *                            stay exactly where they are, like the keypad
 *                            digits and the category chips, where the saving
 *                            compounds across several taps.
 *
 * Both forms work from a keyboard: `click` is what Enter and Space produce,
 * and the fast form keeps an onClick fallback for it.
 */
export function useTap() {
  const handledByPointer = useRef(false);

  return useCallback(
    (handler: () => void, opts: { fast?: boolean } = {}) => {
      // The safe default. A plain onClick cannot ghost-click, because the
      // browser resolves the target before the handler runs.
      if (!opts.fast) {
        return { onClick: handler };
      }

      return {
        onPointerDown: (e: ReactPointerEvent<HTMLButtonElement>) => {
          if (e.button !== 0) return;
          // Keeps the press from stealing focus or starting a selection.
          e.preventDefault();
          handledByPointer.current = true;
          // Self-healing: if the matching click never arrives (finger slid
          // off, gesture cancelled), stop swallowing later clicks.
          setTimeout(() => {
            handledByPointer.current = false;
          }, 400);
          handler();
        },
        onClick: () => {
          // Keyboard activation, assistive tech, or anything that synthesises
          // a click without a pointer sequence.
          if (handledByPointer.current) {
            handledByPointer.current = false;
            return;
          }
          handler();
        },
      };
    },
    [],
  );
}
