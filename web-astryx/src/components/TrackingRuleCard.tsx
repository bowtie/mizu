import { HStack } from "@astryxdesign/core/HStack";
import { VStack } from "@astryxdesign/core/VStack";
import { Card } from "@astryxdesign/core/Card";
import { Text } from "@astryxdesign/core/Text";
import { Switch } from "@astryxdesign/core/Switch";
import { play as cuelumePlay } from "cuelume";
import { memo } from "react";
import type { KeyboardEvent, MouseEvent } from "react";

type RuleVariant = "green" | "yellow" | "red";

interface TrackingRuleCardProps {
  variant: RuleVariant;
  label: string;
  description: string;
  switchLabel: string;
  value: boolean;
  onChange: (value: boolean) => void;
}

export const TrackingRuleCard = memo(function TrackingRuleCard({
  variant,
  label,
  description,
  switchLabel,
  value,
  onChange,
}: TrackingRuleCardProps) {
  // Clicking anywhere on the card toggles the switch.
  // Clicks on the switch itself are handled by the Switch component
  // and won't bubble here (the input has zIndex:1 and stops propagation
  // via the native checkbox change event).
  const handleCardClick = (e: MouseEvent<HTMLDivElement>) => {
    // If the click originated from the switch's input, the Switch's
    // own onChange handles it — don't double-toggle.
    const target = e.target as HTMLElement;
    if (target.tagName === "INPUT" || target.closest("input")) {
      return;
    }
    cuelumePlay("toggle", { volume: 0.5 });
    onChange(!value);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    // Don't intercept keyboard events from the switch — it handles its own.
    const target = e.target as HTMLElement;
    if (target.tagName === "INPUT" || target.closest("input")) {
      return;
    }
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onChange(!value);
    }
  };

  return (
    <Card
      padding={4}
      variant={variant}
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      aria-pressed={value}
      aria-label={`${label}: ${value ? "enabled" : "disabled"}`}
      className="tracking-rule-card"
      style={{ cursor: "pointer" }}
    >
      <HStack hAlign="between" vAlign="center" wrap="nowrap">
        <VStack gap={0}>
          <Text type="body" weight="semibold" display="block">
            {label}
          </Text>
          <Text type="supporting" color="secondary" display="block">
            {description}
          </Text>
        </VStack>
        <div onClick={(e) => e.stopPropagation()}>
          <Switch
            label={switchLabel}
            value={value}
            onChange={(v) => {
              cuelumePlay("toggle", { volume: 0.5 });
              onChange(v);
            }}
            isLabelHidden
          />
        </div>
      </HStack>
    </Card>
  );
});
