import * as React from "react"

const MOBILE_BREAKPOINT = 768

const NARROW = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`

function subscribe(onChange: () => void) {
  const query = window.matchMedia(NARROW)
  query.addEventListener("change", onChange)
  return () => query.removeEventListener("change", onChange)
}

/**
 * Whether the window is narrow enough to treat as a phone.
 *
 * useSyncExternalStore rather than useState + useEffect: the media query is an
 * outside source of truth, and React has a hook for exactly that. The version
 * this replaced returned false on the first render and the real answer on the
 * second, so a narrow window flashed the wide layout before correcting itself.
 */
export function useIsMobile() {
  return React.useSyncExternalStore(
    subscribe,
    () => window.matchMedia(NARROW).matches,
    // The server snapshot. Nothing renders on a server here, but the argument
    // is required and "not a phone" is the right default.
    () => false,
  )
}
