import type { ReactNode } from "react";
import { HStack } from "@astryxdesign/core/HStack";
import { VStack } from "@astryxdesign/core/VStack";
import { Text } from "@astryxdesign/core/Text";

interface SettingsRowProps {
  label: string;
  description: ReactNode;
  children: ReactNode;
  error?: string;
}

export function SettingsRow({ label, description, children, error }: SettingsRowProps) {
  return (
    <HStack hAlign="between" vAlign="center" className="settings-row" wrap="wrap">
      <VStack gap={0} className="settings-row-label">
        <Text type="body" weight="semibold" display="block">
          {label}
        </Text>
        <Text type="supporting" color="secondary" display="block">
          {description}
        </Text>
        {error && (
          <Text type="supporting" display="block" className="settings-row-error">
            {error}
          </Text>
        )}
      </VStack>
      {children}
    </HStack>
  );
}
