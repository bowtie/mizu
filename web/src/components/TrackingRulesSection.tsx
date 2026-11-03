import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Badge } from "@astryxdesign/core/Badge";
import { Button } from "@astryxdesign/core/Button";
import { Save } from "lucide-react";
import { TrackingRuleCard } from "./TrackingRuleCard";

interface TrackingRulesSectionProps {
  rulesDirty: boolean;
  trackAnimeCanon: boolean;
  trackMixed: boolean;
  trackFiller: boolean;
  onAnimeCanonChange: (value: boolean) => void;
  onMixedChange: (value: boolean) => void;
  onFillerChange: (value: boolean) => void;
  saving: boolean;
  /** Saves pending rule and episode changes. Shown with the badge so it is
   *  reachable without opening the actions bar. */
  onSave: () => void;
}

export function TrackingRulesSection({
  rulesDirty,
  trackAnimeCanon,
  trackMixed,
  trackFiller,
  onAnimeCanonChange,
  onMixedChange,
  onFillerChange,
  saving,
  onSave,
}: TrackingRulesSectionProps) {
  return (
    <VStack gap={4}>
      <HStack gap={3} align="center">
        <Heading level={2}>Tracking rules</Heading>
        <div style={{ flex: 1 }} />
        {rulesDirty && (
          <>
            <Badge variant="warning" label="Unsaved changes" />
            <Button
              label="Save"
              variant="primary"
              size="sm"
              icon={<Save size={14} />}
              isLoading={saving}
              isDisabled={saving}
              onClick={onSave}
              data-cuelume-press
            />
          </>
        )}
      </HStack>

      <Grid columns={{ minWidth: 240 }} gap={4} align="start" className="tracking-rules-grid">
        <TrackingRuleCard
          variant="green"
          label="Canon"
          description="Main plot episodes"
          switchLabel="Track canon"
          value={trackAnimeCanon}
          onChange={onAnimeCanonChange}
        />
        <TrackingRuleCard
          variant="yellow"
          label="Mixed"
          description="Mix of canon and filler"
          switchLabel="Track mixed"
          value={trackMixed}
          onChange={onMixedChange}
        />
        <TrackingRuleCard
          variant="red"
          label="Filler"
          description="Skippable, no plot impact"
          switchLabel="Track filler"
          value={trackFiller}
          onChange={onFillerChange}
        />
      </Grid>
    </VStack>
  );
}
