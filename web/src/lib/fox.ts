export interface BadgeMeta {
  label: string;
}

export function syncStateBadge(enabled: boolean): BadgeMeta {
  return enabled ? { label: "Syncing" } : { label: "Paused" };
}
