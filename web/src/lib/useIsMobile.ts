import { useEffect, useState } from "react";

/** Phone width, matching the CSS's `max-width: 600px` breakpoint. */
export const MOBILE_MAX_WIDTH = 600;

/** Whether the viewport is phone-sized; updates on resize. */
export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const update = () => setIsMobile(window.innerWidth <= MOBILE_MAX_WIDTH);
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, []);
  return isMobile;
}
