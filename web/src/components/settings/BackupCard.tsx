import { VStack } from "@astryxdesign/core/VStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { FileDown, FileUp } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import type { RefObject } from "react";

interface BackupCardProps {
  importingConfig: boolean;
  importFileRef: RefObject<HTMLInputElement | null>;
  handleExportConfig: () => Promise<void>;
  handleImportConfigFile: (file: File) => Promise<void>;
}

export function BackupCard({
  importingConfig,
  importFileRef,
  handleExportConfig,
  handleImportConfigFile,
}: BackupCardProps) {
  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Backup</Heading>
        <Divider />
        <SettingsRow
          label="Export config"
          description="Download all shows and settings as a JSON file for backup or migration."
        >
          <Button
            label="Export"
            variant="secondary"
            icon={<FileDown size={16} />}
            onClick={() => void handleExportConfig()}
            data-cuelume-press
          />
        </SettingsRow>
        <Divider />
        <SettingsRow
          label="Import config"
          description="Restore shows and settings from an exported file. Existing shows with the same series are replaced."
        >
          <Button
            label={importingConfig ? "Importing..." : "Import"}
            variant="secondary"
            icon={<FileUp size={16} />}
            isLoading={importingConfig}
            onClick={() => importFileRef.current?.click()}
            data-cuelume-press
          />
        </SettingsRow>
        <input
          ref={importFileRef}
          type="file"
          accept="application/json,.json"
          style={{ display: "none" }}
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) void handleImportConfigFile(file);
          }}
        />
      </VStack>
    </Card>
  );
}
