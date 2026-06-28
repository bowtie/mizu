import { autoAnimate } from '@formkit/auto-animate';

/**
 * Svelte action wrapper for @formkit/auto-animate.
 * Usage: <div use:autoAnimate>
 */
export function autoAnimateAction(node, options) {
  autoAnimate(node, options);
  return {
    update(newOptions) {
      // autoAnimate doesn't support reconfiguration, but it's idempotent
    },
  };
}
