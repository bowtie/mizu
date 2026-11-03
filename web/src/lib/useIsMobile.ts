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

/** Whether the main input can't hover (a touch screen), so hover-only
 *  affordances need a tap instead. */
export function useIsTouch(): boolean {
  const [isTouch, setIsTouch] = useState(false);
  useEffect(() => {
    const query = window.matchMedia("(hover: none)");
    const update = () => setIsTouch(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  return isTouch;
}
