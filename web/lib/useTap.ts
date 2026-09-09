"use client";

import { useCallback } from "react";

/** Use the browser's single activation event for touch, mouse and keyboard.
 * Mixing pointerdown and click needs timing guesses and duplicates long taps.
 * touch-action: manipulation in globals.css removes the double-tap delay.
 */
export function useTap() {
  return useCallback((handler: () => void) => ({ onClick: handler }), []);
}
