import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Switch } from "@astryxdesign/core/Switch";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { SettingsRow } from "../SettingsRow";
import type { SettingsForm } from "../../lib/schemas";

interface InterfaceCardProps {
  control: Control<SettingsForm>;
}

export function InterfaceCard({ control }: InterfaceCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Interface</Heading>
        <Divider />
        <SettingsRow
          label="Interaction sounds"
          description="Play soft sounds when you hover, click, and get notifications."
        >
          <Controller
            control={control}
            name="soundsEnabled"
            render={({ field }) => (
              <Switch
                label="Enable interaction sounds"
                value={field.value}
                onChange={field.onChange}
                isLabelHidden
                data-cuelume-toggle
              />
            )}
          />
        </SettingsRow>
      </VStack>
    </Card>
  );
}
