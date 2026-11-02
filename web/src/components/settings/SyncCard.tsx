import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { RefreshCw } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import { SecretField } from "./SecretField";
import { DefaultsSection } from "./DefaultsSection";
import { formatRelativeTime } from "../../lib/time";
import type { SettingsForm } from "../../lib/schemas";
import type { SelectOption } from "../../lib/types";

export interface SecretKeyState {
  isSet: boolean;
  dirty: boolean;
  savedMasked: string;
}

const cacheStatus = (count: number, refreshedAt: string | null | undefined): string =>
  refreshedAt
    ? `${count} shows cached, refreshed ${formatRelativeTime(refreshedAt)}.`
    : `${count} shows cached. Never refreshed.`;

export const syncIntervalOptions: SelectOption[] = [
  { value: "1h", label: "1 hour" },
  { value: "3h", label: "3 hours" },
  { value: "6h", label: "6 hours" },
  { value: "12h", label: "12 hours" },
  { value: "24h", label: "24 hours" },
];

interface SyncCardProps {
  control: Control<SettingsForm>;
  artworkSourceOptions: SelectOption[];
  tmdb: SecretKeyState;
  fanart: SecretKeyState;
  setTmdbApiKeyDirty: (dirty: boolean) => void;
  setFanartApiKeyDirty: (dirty: boolean) => void;
  cacheCount: number;
  cacheRefreshedAt: string | null;
  refreshingCache: boolean;
  handleRefreshCache: () => void;
}

export function SyncCard({
  control,
  artworkSourceOptions,
  tmdb,
  fanart,
  setTmdbApiKeyDirty,
  setFanartApiKeyDirty,
  cacheCount,
  cacheRefreshedAt,
  refreshingCache,
  handleRefreshCache,
}: SyncCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Sync</Heading>
        <Divider />

        <SettingsRow
          label="Sync interval"
          description="How often Mizu checks for episode changes and updates Sonarr."
        >
          <Controller
            control={control}
            name="syncInterval"
            render={({ field }) => (
              <Selector
                label="Sync interval"
                value={field.value}
                options={syncIntervalOptions}
                onChange={(v) => field.onChange(v as string)}
                isLabelHidden
                width={180}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <DefaultsSection control={control} />
        <Divider />

        {/* Multi-source artwork system */}
        <SettingsRow
          label="Artwork source"
          description="Where artwork comes from by default. Images are saved locally, so browsing works offline."
        >
          <Controller
            control={control}
            name="artworkDefaultSource"
            render={({ field }) => (
              <Selector
                label="Artwork source"
                value={field.value}
                options={artworkSourceOptions}
                onChange={(v) => field.onChange(v as string)}
                isLabelHidden
                width={220}
              />
            )}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="TMDB API key"
          description="Required for TMDB artwork. Get a free key at themoviedb.org."
        >
          <SecretField
            control={control}
            name="tmdbApiKey"
            label="TMDB API key"
            isSet={tmdb.isSet}
            dirty={tmdb.dirty}
            savedMasked={tmdb.savedMasked}
            emptyPlaceholder="Enter TMDB API key"
            width={260}
            onDirtyChange={setTmdbApiKeyDirty}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Fanart.tv API key"
          description="Required for Fanart.tv artwork. Get a free key at fanart.tv."
        >
          <SecretField
            control={control}
            name="fanartApiKey"
            label="Fanart.tv API key"
            isSet={fanart.isSet}
            dirty={fanart.dirty}
            savedMasked={fanart.savedMasked}
            emptyPlaceholder="Enter Fanart.tv API key"
            width={260}
            onDirtyChange={setFanartApiKeyDirty}
          />
        </SettingsRow>
        <Divider />

        <SettingsRow label="AFL cache" description={cacheStatus(cacheCount, cacheRefreshedAt)}>
          <Button
            label={refreshingCache ? "Refreshing..." : "Refresh"}
            variant="secondary"
            icon={<RefreshCw size={16} />}
            onClick={handleRefreshCache}
            isLoading={refreshingCache}
            data-cuelume-press
          />
        </SettingsRow>
      </VStack>
    </Card>
  );
}
