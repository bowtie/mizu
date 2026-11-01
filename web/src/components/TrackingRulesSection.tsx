import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Badge } from "@astryxdesign/core/Badge";
import { TrackingRuleCard } from "./TrackingRuleCard";

interface TrackingRulesSectionProps {
  rulesDirty: boolean;
  trackAnimeCanon: boolean;
  trackMixed: boolean;
  trackFiller: boolean;
  onAnimeCanonChange: (value: boolean) => void;
  onMixedChange: (value: boolean) => void;
  onFillerChange: (value: boolean) => void;
}

export function TrackingRulesSection({
  rulesDirty,
  trackAnimeCanon,
  trackMixed,
  trackFiller,
  onAnimeCanonChange,
  onMixedChange,
  onFillerChange,
}: TrackingRulesSectionProps) {
  return (
    <VStack gap={4}>
      <HStack gap={3} align="center">
        <Heading level={2}>Tracking rules</Heading>
        <div style={{ flex: 1 }} />
        {rulesDirty && <Badge variant="warning" label="Unsaved changes" />}
      </HStack>

      <Grid columns={{ minWidth: 240 }} gap={4} align="start">
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
