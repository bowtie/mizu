import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { Grid } from "@astryxdesign/core/Grid";
import { List, ListItem } from "@astryxdesign/core/List";
import { Search, Check } from "lucide-react";
import { TrackingRuleCard } from "../TrackingRuleCard";
import type { AddWizardState, AddWizardAction } from "../../routes/addWizardHelpers";
import type { SelectOption } from "../../lib/types";
import {
  ADD_STEP_UNSAFE_MATCH,
  START_FROM_BEGINNING,
  START_FROM_CURRENT,
} from "../../routes/addWizardHelpers";
import { isUnsafeMatch, matchSlug, matchTitle } from "./matchHelpers";

const startModeOptions: SelectOption[] = [
  { value: START_FROM_CURRENT, label: "Current progress" },
  { value: START_FROM_BEGINNING, label: "Episode 1" },
];

interface MatchStepProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  handleManualSearch: () => void | Promise<void>;
  handleConfirmMatch: () => void | Promise<void>;
  handlePrimaryMatchAction: () => void | Promise<void>;
  handleKeepSonarrOnly: () => void | Promise<void>;
}

export function MatchStep({
  w,
  dispatch,
  handleManualSearch,
  handleConfirmMatch,
  handlePrimaryMatchAction,
  handleKeepSonarrOnly,
}: MatchStepProps) {
  return (
    <VStack gap={4}>
      {w.matchLoading ? (
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading AFL suggestions" />
        </VStack>
      ) : (
        <>
          {w.manualSearchVisible && (
            <VStack gap={2}>
              <TextInput
                label="Manual AFL search"
                value={w.manualQuery}
                onChange={(v) => dispatch({ type: "SET", payload: { manualQuery: v } })}
                placeholder="Search AnimeFillerList"
                startIcon={Search}
              />
              <Button
                label="Search"
                variant="secondary"
                onClick={handleManualSearch}
                isLoading={w.manualSearching}
                data-cuelume-press
              />
            </VStack>
          )}
          {w.displayedMatches.length > 0 ? (
            <List hasDividers className="wizard-list">
              {w.displayedMatches.map((match) => {
                const isSelected = matchSlug(w.selectedAfl) === matchSlug(match);
                const unsafe = isUnsafeMatch(match);
                return (
                  <ListItem
                    key={matchSlug(match)}
                    label={matchTitle(match)}
                    description={unsafe ? "Needs confirmation" : undefined}
                    onClick={() => dispatch({ type: "SET", payload: { selectedAfl: match } })}
                    endContent={
                      <HStack gap={2} align="center">
                        {match.confidence != null && (
                          <Text type="supporting" hasTabularNumbers>
                            {Math.round(match.confidence * 100)}%
                          </Text>
                        )}
                        {isSelected && <Check size={16} color="var(--color-accent)" />}
                      </HStack>
                    }
                  />
                );
              })}
            </List>
          ) : (
            !w.manualSearchVisible && (
              <EmptyState
                title="No AFL matches"
                description="Try a manual search above."
                icon={<Icon icon={Search} size="lg" color="secondary" />}
              />
            )
          )}

          {/* Tracking rules */}
          <Divider />
          <Card variant="muted" padding={4}>
            <VStack gap={3}>
              <Heading level={3}>Tracking rules</Heading>
              <Grid columns={{ minWidth: 240 }} gap={3}>
                <TrackingRuleCard
                  variant="red"
                  label="Filler"
                  description="Skippable, no plot impact"
                  switchLabel="Track filler"
                  value={w.addTrackFiller}
                  onChange={(v) => dispatch({ type: "SET", payload: { addTrackFiller: v } })}
                />
                <TrackingRuleCard
                  variant="yellow"
                  label="Mixed"
                  description="Mix of canon and filler"
                  switchLabel="Track mixed canon"
                  value={w.addTrackMixed}
                  onChange={(v) => dispatch({ type: "SET", payload: { addTrackMixed: v } })}
                />
                <TrackingRuleCard
                  variant="green"
                  label="Canon"
                  description="Main plot episodes"
                  switchLabel="Track anime canon"
                  value={w.addTrackAnimeCanon}
                  onChange={(v) => dispatch({ type: "SET", payload: { addTrackAnimeCanon: v } })}
                />
              </Grid>
              <Selector
                label="Start tracking from"
                options={startModeOptions}
                value={w.addStartMode}
                onChange={(v) => dispatch({ type: "SET", payload: { addStartMode: v } })}
              />
            </VStack>
          </Card>

          {w.addStep === ADD_STEP_UNSAFE_MATCH ? (
            <Button
              label="Confirm match"
              variant="primary"
              onClick={() => handleConfirmMatch()}
              isLoading={w.confirming}
              data-cuelume-press
            />
          ) : (
            <HStack gap={2}>
              <Button
                label="Keep Sonarr only"
                variant="secondary"
                onClick={handleKeepSonarrOnly}
                data-cuelume-press
              />
              <Button
                label="Confirm match"
                variant="primary"
                onClick={handlePrimaryMatchAction}
                isLoading={w.confirming}
                data-cuelume-press
              />
            </HStack>
          )}
        </>
      )}
    </VStack>
  );
}
