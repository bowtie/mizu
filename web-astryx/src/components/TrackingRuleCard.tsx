import type { ReactNode } from "react";
import { HStack } from "@astryxdesign/core/HStack";
import { VStack } from "@astryxdesign/core/VStack";
import { Card } from "@astryxdesign/core/Card";
import { Text } from "@astryxdesign/core/Text";
import { Switch } from "@astryxdesign/core/Switch";
import type { CSSProperties } from "react";

type RuleVariant = "green" | "yellow" | "red";

const accentVars: Record<RuleVariant, CSSProperties> = {
  green: {
    "--color-accent": "var(--color-border-green)",
    "--color-background-gray": "var(--color-background-green)",
  } as CSSProperties,
  yellow: {
    "--color-accent": "var(--color-border-yellow)",
    "--color-background-gray": "var(--color-background-yellow)",
  } as CSSProperties,
  red: {
    "--color-accent": "var(--color-border-red)",
    "--color-background-gray": "var(--color-background-red)",
  } as CSSProperties,
};

interface TrackingRuleCardProps {
  variant: RuleVariant;
  label: string;
  description: string;
  switchLabel: string;
  value: boolean;
  onChange: (value: boolean) => void;
}

export function TrackingRuleCard({
  variant,
  label,
  description,
  switchLabel,
  value,
  onChange,
}: TrackingRuleCardProps) {
  return (
    <Card padding={4} variant={variant}>
      <HStack hAlign="between" vAlign="center" wrap="nowrap">
        <VStack gap={0} className="tracking-rule-label">
          <Text type="body" weight="semibold" display="block">
            {label}
          </Text>
          <Text type="supporting" color="secondary" display="block">
            {description}
          </Text>
        </VStack>
        <Switch
          label={switchLabel}
          value={value}
          onChange={onChange}
          isLabelHidden
          style={accentVars[variant]}
        />
      </HStack>
    </Card>
  );
}
