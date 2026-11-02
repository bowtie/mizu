import { memo, useEffect, useState } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { Card } from "@astryxdesign/core/Card";
import { ToggleButton } from "@astryxdesign/core/ToggleButton";
import { Collapsible } from "@astryxdesign/core/Collapsible";
import { List, ListItem } from "@astryxdesign/core/List";
import { SegmentedControl, SegmentedControlItem } from "@astryxdesign/core/SegmentedControl";
import {
  Eye,
  EyeOff,
  BookOpen,
  Tv,
  Shuffle,
  SkipForward,
  CircleAlert,
  LoaderCircle,
} from "lucide-react";
import type { Episode } from "../lib/types";
import {
  displayEpisodeNumber,
  episodeBadgeInfo,
  epSyncKey,
  seasonLabel,
  type SeasonGroup,
} from "../routes/showDetailHelpers";
import { VirtualizedEpisodeList } from "./VirtualizedEpisodeList";

interface EpisodeListSectionProps {
  episodesLoading: boolean;
  episodesError: string | null;
  onRetryEpisodes: () => void;
  hasShowData: boolean;
  groupedBySeason: SeasonGroup[];
  /** True while the deferred list lags behind the selected filter. */
  listStale: boolean;
  epStats: { total: number; canon: number; anime: number; mixed: number; filler: number };
  activeTypeFilter: string;
  syncingEps: Record<string, boolean>;
  onFilterChange: (value: string) => void;
  onToggleMonitor: (groupSeason: number, ep: Episode) => void;
  isMobile: boolean;
}

/** Single episode row — memoized so that toggling one episode's monitor
 *  state doesn't re-render all 1200+ ListItems. Only re-renders when
 *  the episode data or its sync state changes. */
const EpisodeListItem = memo(function EpisodeListItem({
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
  return (
    <ListItem
      key={`${groupSeason}-${epNum}`}
      label={ep.title || "Untitled"}
      description={`Episode ${epNum}`}
      endContent={
        <HStack gap={4} align="center">
          {ep.air_date && (
            <Text type="supporting" size="sm" color="secondary">
              {new Date(ep.air_date).toLocaleDateString(undefined, {
                year: "numeric",
                month: "short",
                day: "numeric",
              })}
            </Text>
          )}
          <Badge variant={badge.variant} label={badge.label} />
          <ToggleButton
            label={monitored ? "Unmonitor episode" : "Monitor episode"}
            isIconOnly
            size="sm"
            icon={<Eye size={14} />}
            pressedIcon={<EyeOff size={14} />}
            isPressed={monitored}
            isDisabled={isSyncing || !ep.sonarr_episode_id}
            isLoading={isSyncing}
            onPressedChange={() => onToggleMonitor(groupSeason, ep)}
            aria-label={monitored ? "Unmonitor" : "Monitor"}
          />
        </HStack>
      }
    />
  );
});

/** Episode list section — memoized so tracking-rule toggles don't re-render
 *  the (potentially 1200+) episode list. Only re-renders when episode data,
 *  filter, or syncing state actually changes. */
export const EpisodeListSection = memo(function EpisodeListSection({
  episodesLoading,
  episodesError,
  onRetryEpisodes,
  hasShowData,
  groupedBySeason,
  listStale,
  epStats,
  activeTypeFilter,
  syncingEps,
  onFilterChange,
  onToggleMonitor,
  isMobile,
}: EpisodeListSectionProps) {
  // Short-circuit: if no episodes are syncing, skip the epSyncKey
  // computation for every item (83+ per expanded season). syncingEps
  // is only non-empty during a brief monitor toggle.
  const hasSyncing = Object.keys(syncingEps).length > 0;

  // Track which seasons have been expanded at least once — collapsed
  // seasons that were never opened skip rendering their (potentially
  // 1000+) episode ListItems entirely, saving DOM nodes.
  // Initialize with the default-open season (last one) so it renders
  // its episodes on the first paint — no extra useEffect round-trip.
  const [openedSeasons, setOpenedSeasons] = useState<Set<string>>(() => {
    if (groupedBySeason.length === 0) return new Set();
    const last = groupedBySeason[groupedBySeason.length - 1];
    return new Set([String(last.season)]);
  });

  // When groupedBySeason changes (filter switch), auto-open the new
  // last season and clean up stale seasons that no longer exist.
  useEffect(() => {
    if (groupedBySeason.length === 0) {
      setOpenedSeasons(new Set());
      return;
    }
    const validSeasons = new Set(groupedBySeason.map((g) => String(g.season)));
    const lastKey = String(groupedBySeason[groupedBySeason.length - 1].season);
    setOpenedSeasons((prev) => {
      const next = new Set<string>();
      for (const key of prev) {
        if (validSeasons.has(key)) next.add(key);
      }
      next.add(lastKey);
      return next;
    });
  }, [groupedBySeason]);

  // Types with no episodes are not offered as tabs. The active one stays so
  // the control never loses its selection.
  const typeTabs = [
    {
      value: "canon",
      label: "Manga",
      count: isMobile ? epStats.canon + epStats.anime : epStats.canon,
      icon: <BookOpen size={16} />,
    },
    { value: "anime_canon", label: "Anime", count: epStats.anime, icon: <Tv size={16} /> },
    { value: "mixed_canon", label: "Mixed", count: epStats.mixed, icon: <Shuffle size={16} /> },
    {
      value: "filler",
      label: "Filler",
      count: epStats.filler,
      icon: <SkipForward size={16} />,
    },
  ].filter((tab) => tab.count > 0 || tab.value === activeTypeFilter);

  const filterItems = (
    <>
      <SegmentedControlItem value="all" label={`All (${epStats.total})`} />
      {typeTabs.map((tab) => (
        <SegmentedControlItem
          key={tab.value}
          value={tab.value}
          label={`${tab.label} (${tab.count})`}
          icon={tab.icon}
        />
      ))}
    </>
  );

  return (
    <VStack gap={4}>
      <HStack gap={3} align="center">
        <Heading level={2}>Episode list</Heading>
        <div style={{ flex: 1 }} />
      </HStack>

      {/* Episode filters */}
      <SegmentedControl
        value={activeTypeFilter}
        onChange={onFilterChange}
        label="Episode type filter"
        layout="fill"
        size="lg"
      >
        {filterItems}
      </SegmentedControl>

      {/* Episodes by season — keep skeleton visible while useDeferredValue
          catches up to avoid a flash of "No episodes" empty state. */}
      {episodesError && !episodesLoading ? (
        <EmptyState
          title="Couldn't load episodes"
          description={episodesError}
          icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
          actions={
            <Button
              label="Try again"
              variant="secondary"
              onClick={onRetryEpisodes}
              data-cuelume-press
            />
          }
        />
      ) : episodesLoading || listStale ? (
        hasShowData ? (
          /* Minimal loader — show data is already rendered, just waiting
             for episodes to arrive from prefetch/network. */
          <EmptyState
            title="Loading episodes…"
            description="Fetching episode data from Sonarr."
            icon={<Icon icon={LoaderCircle} size="lg" color="secondary" className="spin" />}
          />
        ) : (
          <VStack gap={4}>
            {/* Skeleton filter bar */}
            <div style={{ display: "flex", gap: "var(--spacing-2)" }}>
              <div
                className="skeleton"
                style={{ width: 70, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 80, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 75, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 70, height: 36, borderRadius: "var(--radius-inner)" }}
              />
              <div
                className="skeleton"
                style={{ width: 65, height: 36, borderRadius: "var(--radius-inner)" }}
              />
            </div>
            {/* Skeleton season cards — show 2, not 4, to reduce visual noise */}
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="skeleton-season-card">
                <div className="skeleton" style={{ width: 20, height: 20, borderRadius: 4 }} />
                <div className="skeleton-text" style={{ width: 90 }} />
                <div className="skeleton" style={{ width: 80, height: 20, borderRadius: 10 }} />
                <div style={{ flex: 1 }} />
                <div className="skeleton" style={{ width: 24, height: 24, borderRadius: 4 }} />
              </div>
            ))}
          </VStack>
        )
      ) : groupedBySeason.length === 0 ? (
        <EmptyState
          title="No episodes"
          description="No episodes match the selected type."
          icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
          actions={
            <Button
              label="Show all"
              variant="secondary"
              onClick={() => onFilterChange("all")}
              data-cuelume-press
            />
          }
        />
      ) : (
        <VStack gap={4}>
          {groupedBySeason.map((group) => (
            <Card key={group.season} padding={4} variant="muted" className="season-card">
              <Collapsible
                trigger={
                  <>
                    <span>{seasonLabel(group.season)}</span>
                    <Badge variant="neutral" label={`${group.episodes.length} episodes`} />
                  </>
                }
                value={String(group.season)}
                defaultIsOpen={group.index === groupedBySeason.length - 1}
                onOpenChange={(isOpen) => {
                  if (isOpen) {
                    setOpenedSeasons((prev) => {
                      if (prev.has(String(group.season))) return prev;
                      const next = new Set(prev);
                      next.add(String(group.season));
                      return next;
                    });
                  }
                }}
              >
                {openedSeasons.has(String(group.season)) ||
                group.index === groupedBySeason.length - 1 ? (
                  group.episodes.length > 100 ? (
                    <VirtualizedEpisodeList
                      episodes={group.episodes}
                      groupSeason={group.season}
                      syncingEps={syncingEps}
                      hasSyncing={hasSyncing}
                      onToggleMonitor={onToggleMonitor}
                    />
                  ) : (
                    <div className="episode-list-wrap">
                      <List density="balanced" hasDividers>
                        {group.episodes.map((ep) => (
                          <EpisodeListItem
                            key={`${group.season}-${displayEpisodeNumber(ep)}-${ep.sonarr_episode_id}`}
                            ep={ep}
                            groupSeason={group.season}
                            isSyncing={hasSyncing && !!syncingEps[epSyncKey(group.season, ep)]}
                            onToggleMonitor={onToggleMonitor}
                          />
                        ))}
                      </List>
                    </div>
                  )
                ) : null}
              </Collapsible>
            </Card>
          ))}
        </VStack>
      )}
    </VStack>
  );
});
