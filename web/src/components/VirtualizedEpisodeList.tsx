import { useRef, useCallback } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { HStack } from "@astryxdesign/core/HStack";
import { Text } from "@astryxdesign/core/Text";
import { Badge } from "@astryxdesign/core/Badge";
import { ToggleButton } from "@astryxdesign/core/ToggleButton";
import { Eye, EyeOff } from "lucide-react";
import { memo } from "react";
import type { Episode } from "../lib/types";
import { formatAirDate } from "../routes/showDetailHelpers";

// ── Helpers (duplicated from ShowDetailPage to keep this self-contained) ──

function isSpecialSeasonNumber(num: number | undefined): boolean {
  return num === -1 || num === 0;
}
function displayEpisodeNumber(ep: Episode): number {
  if (isSpecialSeasonNumber(ep.season_number)) return ep.sonarr_episode_number ?? 0;
  return ep.absolute_episode_number ?? ep.episode_num ?? ep.sonarr_episode_number ?? 0;
}
function episodeBadgeInfo(type: string | undefined | null): {
  label: string;
  variant: "neutral" | "blue" | "purple" | "yellow" | "red";
} {
  switch (type) {
    case "canon":
      return { label: "Manga", variant: "blue" };
    case "anime_canon":
      return { label: "Anime", variant: "purple" };
    case "mixed_canon":
    case "mixed":
      return { label: "Mixed", variant: "yellow" };
    case "filler":
      return { label: "Filler", variant: "red" };
    case "special":
      return { label: "Special", variant: "neutral" };
    default:
      return { label: "Unknown", variant: "neutral" };
  }
}

// ── Virtualized row — renders the same content as EpisodeListItem but as a
//    plain div instead of <li>, so it works inside the virtualizer's scroll
//    container without needing a <ul> parent. ──

const VirtualEpisodeRow = memo(function VirtualEpisodeRow({
  ep,
  groupSeason,
  isSyncing,
  onToggleMonitor,
}: {
  ep: Episode;
  groupSeason: number;
  isSyncing: boolean;
  onToggleMonitor: (groupSeason: number, ep: Episode) => void;
}) {
  const badge = episodeBadgeInfo(ep.type);
  const monitored = !!ep.sonarr_episode_id && !!ep.monitored;
  const epNum = displayEpisodeNumber(ep);
  const airDate = formatAirDate(ep.air_date);
  return (
    <div className="virtual-episode-row" role="listitem">
      <div className="virtual-episode-row-content">
        <span className="virtual-episode-label">{ep.title || "Untitled"}</span>
        <span className="virtual-episode-desc">
          Episode {epNum}
          {airDate && (
            <span className="episode-date-inline"> · {formatAirDate(ep.air_date, true)}</span>
          )}
        </span>
      </div>
      <HStack gap={4} align="center" className="episode-row-end">
        {airDate && (
          <Text type="supporting" size="sm" color="secondary" className="episode-date-end">
            {airDate}
          </Text>
        )}
        <Badge variant={badge.variant} label={badge.label} />
        <ToggleButton
          label={monitored ? "Unmonitor episode" : "Monitor episode"}
          isIconOnly
          size="sm"
          icon={<EyeOff size={14} />}
          pressedIcon={<Eye size={14} />}
          className="episode-monitor-toggle"
          isPressed={monitored}
          isDisabled={isSyncing || !ep.sonarr_episode_id}
          isLoading={isSyncing}
          onPressedChange={() => onToggleMonitor(groupSeason, ep)}
        />
      </HStack>
    </div>
  );
});

// ── VirtualizedEpisodeList — wraps useVirtualizer to render only visible
//    episode rows. For seasons with 1000+ episodes (Detective Conan, One
//    Piece), this avoids creating 1000+ DOM nodes and keeps scrolling smooth. ──

interface VirtualizedEpisodeListProps {
  episodes: Episode[];
  groupSeason: number;
  syncingEps: Record<string, boolean>;
  hasSyncing: boolean;
  onToggleMonitor: (groupSeason: number, ep: Episode) => void;
}

function epSyncKey(groupSeason: number, ep: Episode): string {
  return `${groupSeason}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id ?? "x"}`;
}

export const VirtualizedEpisodeList = memo(function VirtualizedEpisodeList({
  episodes,
  groupSeason,
  syncingEps,
  hasSyncing,
  onToggleMonitor,
}: VirtualizedEpisodeListProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  const virtualizer = useVirtualizer({
    count: episodes.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 52,
    overscan: 8,
  });

  const items = virtualizer.getVirtualItems();

  // Measure each row on mount so the virtualizer adjusts from the
  // initial 52px estimate to the actual rendered height.
  const measureElement = useCallback(
    (el: HTMLElement | null) => {
      if (el) virtualizer.measureElement(el);
    },
    [virtualizer],
  );

  return (
    <div className="episode-list-wrap">
      <div
        ref={scrollRef}
        className="virtual-episode-scroll"
        role="list"
        style={{
          height: `${Math.min(virtualizer.getTotalSize(), 600)}px`,
          overflow: "auto",
        }}
      >
        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            position: "relative",
          }}
        >
          {items.map((virtualItem) => {
            const ep = episodes[virtualItem.index];
            return (
              <div
                key={`${groupSeason}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id}`}
                data-index={virtualItem.index}
                ref={measureElement}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${virtualItem.start}px)`,
                }}
              >
                <VirtualEpisodeRow
                  ep={ep}
                  groupSeason={groupSeason}
                  isSyncing={hasSyncing && !!syncingEps[epSyncKey(groupSeason, ep)]}
                  onToggleMonitor={onToggleMonitor}
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
});
