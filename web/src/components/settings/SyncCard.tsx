import { Controller, type Control } from "react-hook-form";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Selector } from "@astryxdesign/core/Selector";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { RefreshCw, Eye, EyeOff } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import type { SettingsForm } from "../../lib/schemas";
import type { SelectOption } from "../../lib/types";

export interface SecretKeyState {
  visible: boolean;
  isSet: boolean;
  dirty: boolean;
  savedMasked: string;
}

const formatCacheTime = (value: string | null | undefined): string => {
  if (!value) return "Never refreshed";
  return new Date(value).toLocaleString();
};

const syncIntervalOptions: SelectOption[] = [
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
  toggleTmdbApiKeyVisibility: () => void;
  toggleFanartApiKeyVisibility: () => void;
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
  toggleTmdbApiKeyVisibility,
  toggleFanartApiKeyVisibility,
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

        {/* Multi-source artwork system */}
        <SettingsRow
          label="Artwork source"
          description="Default provider for fetching show artwork. Images are downloaded and stored locally so browsing works offline."
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
          <HStack gap={2} align="center">
            <Controller
              control={control}
              name="tmdbApiKey"
              render={({ field }) => (
                <TextInput
                  label="TMDB API key"
                  type={tmdb.visible ? "text" : "password"}
                  value={field.value}
                  onChange={(v) => {
                    field.onChange(v);
                    setTmdbApiKeyDirty(true);
                  }}
                  placeholder={
                    tmdb.isSet ? tmdb.savedMasked || "Saved API key" : "Enter TMDB API key"
                  }
                  isLabelHidden
                  width={260}
                />
              )}
            />
            {(tmdb.isSet || tmdb.dirty) && (
              <Button
                label={tmdb.visible ? "Hide" : "Show"}
                variant="ghost"
                isIconOnly
                icon={tmdb.visible ? <EyeOff size={16} /> : <Eye size={16} />}
                onClick={toggleTmdbApiKeyVisibility}
                data-cuelume-toggle="whisper"
              />
            )}
          </HStack>
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="Fanart.tv API key"
          description="Required for Fanart.tv artwork. Get a free key at fanart.tv."
        >
          <HStack gap={2} align="center">
            <Controller
              control={control}
              name="fanartApiKey"
              render={({ field }) => (
                <TextInput
                  label="Fanart.tv API key"
                  type={fanart.visible ? "text" : "password"}
                  value={field.value}
                  onChange={(v) => {
                    field.onChange(v);
                    setFanartApiKeyDirty(true);
                  }}
                  placeholder={
                    fanart.isSet ? fanart.savedMasked || "Saved API key" : "Enter Fanart.tv API key"
                  }
                  isLabelHidden
                  width={260}
                />
              )}
            />
            {(fanart.isSet || fanart.dirty) && (
              <Button
                label={fanart.visible ? "Hide" : "Show"}
                variant="ghost"
                isIconOnly
                icon={fanart.visible ? <EyeOff size={16} /> : <Eye size={16} />}
                onClick={toggleFanartApiKeyVisibility}
                data-cuelume-toggle="whisper"
              />
            )}
          </HStack>
        </SettingsRow>
        <Divider />

        <SettingsRow
          label="AFL cache"
          description={`${cacheCount} shows cached, refreshed ${formatCacheTime(cacheRefreshedAt)}.`}
        >
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
