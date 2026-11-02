import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { Grid } from "@astryxdesign/core/Grid";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { TrackingRuleCard } from "../TrackingRuleCard";
import type { SettingsForm } from "../../lib/schemas";

interface DefaultsSectionProps {
  control: Control<SettingsForm>;
}

export function DefaultsSection({ control }: DefaultsSectionProps) {
  return (
    <VStack gap={4}>
      <VStack gap={1}>
        <Heading level={3}>Tracking defaults</Heading>
        <Text type="supporting" color="secondary" display="block">
          Applied to new anime. You can change them per anime.
        </Text>
      </VStack>

      <Grid columns={{ minWidth: 240, max: 3 }} gap={4} align="start">
        <Controller
          control={control}
          name="trackAnimeCanonDefault"
          render={({ field }) => (
            <TrackingRuleCard
              variant="green"
              label="Canon"
              description="Main plot episodes"
              switchLabel="Track canon"
              value={field.value}
              onChange={field.onChange}
            />
          )}
        />
        <Controller
          control={control}
          name="trackMixedDefault"
          render={({ field }) => (
            <TrackingRuleCard
              variant="yellow"
              label="Mixed"
              description="Mix of canon and filler"
              switchLabel="Track mixed"
              value={field.value}
              onChange={field.onChange}
            />
          )}
        />
        <Controller
          control={control}
          name="trackFillerDefault"
          render={({ field }) => (
            <TrackingRuleCard
              variant="red"
              label="Filler"
              description="Skippable, no plot impact"
              switchLabel="Track filler"
              value={field.value}
              onChange={field.onChange}
            />
          )}
        />
      </Grid>
    </VStack>
  );
}
