import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Card } from "@astryxdesign/core/Card";
import { Selector } from "@astryxdesign/core/Selector";
import { RefreshCw, Image as ImageIcon } from "lucide-react";
import type { SelectOption, Show } from "../lib/types";

interface ArtworkCardProps {
  show: Show;
  sourceOptions: SelectOption[];
  canPick: boolean;
  refreshing: boolean;
  onChoose: () => void;
  onRefresh: () => void;
  onSourceChange: (source: string) => void;
}

export function ArtworkCard({
  show,
  sourceOptions,
  canPick,
  refreshing,
  onChoose,
  onRefresh,
  onSourceChange,
}: ArtworkCardProps) {
  return (
    <Card padding={4} variant="muted" className="artwork-card">
      <VStack gap={3}>
        <HStack gap={3} align="center">
          <ImageIcon size={16} color="var(--color-icon-secondary)" />
          <Heading level={3}>Artwork</Heading>
          <div style={{ flex: 1 }} />
          {canPick && (
            <Button
              label="Choose"
              variant="secondary"
              size="sm"
              icon={<ImageIcon size={14} />}
              onClick={() => onChoose()}
              isDisabled={refreshing}
              data-cuelume-press
            />
          )}
          <Button
            label={refreshing ? "Refreshing..." : "Refresh"}
            variant="ghost"
            size="sm"
            icon={<RefreshCw size={14} className={refreshing ? "spin" : ""} />}
            onClick={() => onRefresh()}
            isDisabled={refreshing}
            data-cuelume-press
          />
        </HStack>
        <HStack gap={3} align="center" wrap="wrap">
          <Text type="supporting" color="secondary">
            Source:
          </Text>
          <Selector
            label="Artwork source"
            value={show.artwork_source || ""}
            options={sourceOptions}
            onChange={(v) => onSourceChange(v as string)}
            isLabelHidden
            width={220}
          />
          {show.artwork_source && show.artwork_source !== "" && (
            <Badge variant="info" label={`Using ${show.artwork_source}`} />
          )}
        </HStack>
      </VStack>
    </Card>
  );
}
