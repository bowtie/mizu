export const episodeTypeMeta = {
  canon: { label: "Manga", variant: "default" },
  mixed_canon: { label: "Mixed", variant: "warning" },
  mixed: { label: "Mixed", variant: "warning" },
  anime_canon: { label: "Anime", variant: "info" },
  filler: { label: "Filler", variant: "destructive" },
  special: { label: "Special", variant: "secondary" },
  unknown: { label: "Unknown", variant: "secondary" },
};

export function episodeTypeBadge(type) {
  return episodeTypeMeta[type] || episodeTypeMeta.unknown;
}

export function runStatusBadge(status) {
  switch (status) {
    case "success":
      return { label: "Success", variant: "success" };
    case "partial":
      return { label: "Partial", variant: "warning" };
    case "failed":
      return { label: "Failed", variant: "destructive" };
    case "running":
      return { label: "Running", variant: "info" };
    default:
      return { label: status || "Unknown", variant: "secondary" };
  }
}

export function itemStatusBadge(status) {
  switch (status) {
    case "success":
      return { label: "Success", variant: "success" };
    case "failed":
      return { label: "Failed", variant: "destructive" };
    default:
      return { label: status || "Info", variant: "secondary" };
  }
}

export function syncStateBadge(enabled) {
  return enabled
    ? { label: "Syncing", variant: "success" }
    : { label: "Paused", variant: "secondary" };
}

export function monitorStateBadge(monitored, yesLabel = "Monitored", noLabel = "Unmonitored") {
  return monitored
    ? { label: yesLabel, variant: "success" }
    : { label: noLabel, variant: "secondary" };
}
