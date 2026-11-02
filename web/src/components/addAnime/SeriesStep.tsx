import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { TextInput } from "@astryxdesign/core/TextInput";
import { List, ListItem } from "@astryxdesign/core/List";
import { Search, Check, CircleAlert } from "lucide-react";
import type { SonarrSeries } from "../../lib/types";
import type { AddWizardState, AddWizardAction } from "../../routes/addWizardHelpers";

interface SeriesStepProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  allSeries: SonarrSeries[];
  filteredSeriesLibrary: SonarrSeries[];
  mappedSeriesIds: Set<string | number>;
  posterUrl: (series: SonarrSeries | null | undefined) => string;
  openExistingMapping: (seriesId: string | number) => void;
  goToFallbackAflStep: (prefill?: string) => void | Promise<void>;
  goToMatchStep: () => void | Promise<void>;
}

export function SeriesStep({
  w,
  dispatch,
  allSeries,
  filteredSeriesLibrary,
  mappedSeriesIds,
  posterUrl,
  openExistingMapping,
  goToFallbackAflStep,
  goToMatchStep,
}: SeriesStepProps) {
  return w.sonarrLoading ? (
    <VStack gap={3} hAlign="center" padding={6}>
      <Spinner size="lg" label="Loading available Sonarr anime" />
    </VStack>
  ) : allSeries.length === 0 ? (
    <EmptyState
      title="No anime found in Sonarr"
      description="Sonarr doesn't have any anime series yet."
      icon={<Icon icon={CircleAlert} size="lg" color="secondary" />}
    />
  ) : (
    <VStack gap={4}>
      <TextInput
        label="Search Sonarr anime"
        isLabelHidden
        value={w.seriesQuery}
        onChange={(v) => dispatch({ type: "SET", payload: { seriesQuery: v } })}
        placeholder="Search Sonarr anime"
        startIcon={Search}
      />
      {filteredSeriesLibrary.length === 0 ? (
        <EmptyState
          title="No anime match that search"
          description="Try a different title or year, or add it from the AFL list instead."
          icon={<Icon icon={Search} size="lg" color="secondary" />}
          actions={
            <Button
              label="Not in Sonarr? Search AFL and add to Sonarr"
              variant="secondary"
              icon={<Search size={16} />}
              onClick={() => goToFallbackAflStep(w.seriesQuery.trim())}
              data-cuelume-press
            />
          }
        />
      ) : (
        <List hasDividers className="wizard-list">
          {filteredSeriesLibrary.map((series) => {
            const seriesMapped = mappedSeriesIds.has(series.id);
            const manualPending = !!series.manualMatchPending;
            const isSelected = w.selectedSonarr?.id === series.id;
            return (
              <ListItem
                key={series.id}
                label={series.title}
                description={`${series.year || "Anime"}${series.statistics?.episodeCount ? ` · ${series.statistics.episodeCount} eps` : ""}${manualPending ? " · Needs match" : seriesMapped ? " · Already added" : ""}`}
                onClick={() => {
                  if (seriesMapped) {
                    openExistingMapping(series.id);
                    return;
                  }
                  dispatch({ type: "SET", payload: { selectedSonarr: series } });
                }}
                startContent={
                  posterUrl(series) ? (
                    <div className="wizard-poster-wrap">
                      <div className="wizard-poster-placeholder" />
                      <img
                        src={posterUrl(series)}
                        alt=""
                        loading="lazy"
                        className="wizard-poster-thumb"
                        onLoad={(e) => e.currentTarget.classList.add("is-loaded")}
                      />
                    </div>
                  ) : (
                    <div className="wizard-poster-wrap">
                      <div className="wizard-poster-placeholder" />
                    </div>
                  )
                }
                endContent={
                  seriesMapped ? (
                    <Text type="supporting">Open</Text>
                  ) : manualPending ? (
                    <Text type="supporting" color="accent">
                      Match
                    </Text>
                  ) : isSelected ? (
                    <Check size={16} color="var(--color-accent)" />
                  ) : undefined
                }
              />
            );
          })}
        </List>
      )}
      <HStack gap={2} justify="between" align="center" wrap="wrap">
        <Button
          label="Not in Sonarr? Add from AFL"
          variant="secondary"
          onClick={() => goToFallbackAflStep(w.seriesQuery.trim())}
          data-cuelume-press
        />
        <Button
          label="Continue"
          variant="primary"
          onClick={goToMatchStep}
          isDisabled={!w.selectedSonarr || mappedSeriesIds.has(w.selectedSonarr.id)}
          data-cuelume-press
        />
      </HStack>
    </VStack>
  );
}
