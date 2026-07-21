import { autoAnimate } from '@formkit/auto-animate';
import type { ActionReturn, Action } from 'svelte/action';

export const autoAnimateAction: Action<HTMLElement, any | undefined> = (node, options?) => {
  autoAnimate(node, options);
  return {
    update(_newOptions: any) {
      // autoAnimate does not support reconfiguration; re-running is idempotent
    },
  } satisfies ActionReturn;
};
