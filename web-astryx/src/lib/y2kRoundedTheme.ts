import { defineTheme } from "@astryxdesign/core/theme";
import { y2kTheme } from "@astryxdesign/theme-y2k";

/** Y2K colors but with rounded corners instead of sharp/brutalist. */
export const y2kRoundedTheme = defineTheme({
  name: "y2k-rounded",
  extends: y2kTheme,
  radius: { base: 6, multiplier: 1 },
  tokens: {
    "--radius-none": "0px",
    "--radius-inner": "4px",
    "--radius-element": "8px",
    "--radius-container": "12px",
    "--radius-page": "16px",
    "--radius-full": "9999px",
  },
  components: {
    button: {
      base: {
        borderRadius: "8px",
        borderWidth: "1px",
        borderStyle: "solid",
        borderColor: "var(--color-border)",
      },
    },
    badge: {
      base: {
        borderRadius: "9999px",
      },
    },
    card: {
      base: {
        borderRadius: "12px",
      },
    },
    section: {
      base: {
        borderRadius: "12px",
      },
    },
  },
});
