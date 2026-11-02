import { useState } from "react";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Switch } from "@astryxdesign/core/Switch";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Heading } from "@astryxdesign/core/Heading";
import { Button } from "@astryxdesign/core/Button";
import { Divider } from "@astryxdesign/core/Divider";
import { Card } from "@astryxdesign/core/Card";
import { FileDown, FileUp } from "lucide-react";
import { SettingsRow } from "../SettingsRow";
import { useAuthStatus } from "../../lib/queries";
import type { RefObject } from "react";

interface BackupCardProps {
  importingConfig: boolean;
  importFileRef: RefObject<HTMLInputElement | null>;
  handleExportConfig: (includeSecrets: boolean, password: string) => Promise<void>;
  handleImportConfigFile: (file: File) => Promise<void>;
}

export function BackupCard({
  importingConfig,
  importFileRef,
  handleExportConfig,
  handleImportConfigFile,
}: BackupCardProps) {
  const { data: status } = useAuthStatus();
  // Behind a sign-in proxy the proxy vouches for the user; otherwise
  // exporting secrets takes the dashboard password.
  const needsPassword = !status?.external;
  const [includeSecrets, setIncludeSecrets] = useState(false);
  const [password, setPassword] = useState("");
  const [exporting, setExporting] = useState(false);

  const exportNow = async () => {
    setExporting(true);
    try {
      await handleExportConfig(includeSecrets, password);
    } finally {
      setPassword("");
      setExporting(false);
    }
  };

  return (
    <Card variant="muted" padding={4}>
      <VStack gap={3}>
        <Heading level={2}>Backup</Heading>
        <Divider />
        <SettingsRow
          label="Export config"
          description="Download your shows and settings as a JSON file. API keys and tokens are left out unless you include them."
        >
          <Button
            label="Export"
            variant="secondary"
            icon={<FileDown size={16} />}
            isLoading={exporting}
            isDisabled={exporting || (includeSecrets && needsPassword && !password)}
            onClick={() => void exportNow()}
            data-cuelume-press
          />
        </SettingsRow>
        <SettingsRow
          label="Include API keys and tokens"
          description="Needed to restore without re-entering them. Keep a file like this private."
        >
          <Switch
            label="Include API keys and tokens"
            value={includeSecrets}
            isLabelHidden
            onChange={(on: boolean) => {
              setIncludeSecrets(on);
              setPassword("");
            }}
            data-cuelume-toggle
          />
        </SettingsRow>
        {includeSecrets && needsPassword && (
          <HStack maxWidth={420}>
            <TextInput
              type="password"
              label="Dashboard password"
              value={password}
              onChange={setPassword}
              isDisabled={exporting}
            />
          </HStack>
        )}
        <Divider />
        <SettingsRow
          label="Import config"
          description="Restore shows and settings from an exported file. Shows you already have are replaced."
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
