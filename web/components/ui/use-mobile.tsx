import * as React from "react"

const MOBILE_BREAKPOINT = 768

export function useIsMobile() {
  const [isMobile, setIsMobile] = React.useState<boolean>(false)

  React.useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`)
    const onChange = () => {
      setIsMobile(window.innerWidth < MOBILE_BREAKPOINT)
    }
    
    // Safari ≤ 14 compatibility: use addListener if addEventListener is not available
    if (mql.addEventListener) {
      mql.addEventListener("change", onChange)
    } else {
      // Fallback for older Safari versions
      mql.addListener(onChange)
    }
    
    setIsMobile(window.innerWidth < MOBILE_BREAKPOINT)
    
    return () => {
      // Safari ≤ 14 compatibility: use removeListener if removeEventListener is not available
      if (mql.removeEventListener) {
        mql.removeEventListener("change", onChange)
      } else {
        // Fallback for older Safari versions
        mql.removeListener(onChange)
      }
    }
  }, [])

  return isMobile
}
