import { useEffect, useState } from "react";
import { Controller, type Control } from "react-hook-form";
import { HStack } from "@astryxdesign/core/HStack";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Eye, EyeOff } from "lucide-react";
import type { SettingsForm } from "../../lib/schemas";
import { useIsMobile } from "../../lib/useIsMobile";

type SecretFieldName = "sonarrApiKey" | "tmdbApiKey" | "fanartApiKey" | "ntfyToken";

interface SecretFieldProps {
  control: Control<SettingsForm>;
  name: SecretFieldName;
  label: string;
  /** A value is already stored server-side. */
  isSet: boolean;
  /** The user has typed a replacement this session. */
  dirty: boolean;
  /** Masked form of the stored value (a fixed placeholder, never part of the key), if the server sent one. */
  savedMasked: string;
  emptyPlaceholder: string;
  width: number;
  onDirtyChange: (dirty: boolean) => void;
  /** Called on every keystroke, e.g. to drop a stale test result. */
  onEdit?: () => void;
}

/**
 * A stored secret is shown masked and read-only with a Replace button, so a
 * stray keystroke can never overwrite it. Replace opens an empty input; Cancel
 * (or a save/discard that clears `dirty`) returns to the masked display.
 */
export function SecretField({
  control,
  name,
  label,
  isSet,
  dirty,
  savedMasked,
  emptyPlaceholder,
  width,
  onDirtyChange,
  onEdit,
}: SecretFieldProps) {
  const [replacing, setReplacing] = useState(false);
  const [visible, setVisible] = useState(false);
  const isMobile = useIsMobile();

  // A save or discard resets `dirty`; drop back to the masked display with it.
  useEffect(() => {
    if (!dirty) {
      setReplacing(false);
      setVisible(false);
    }
  }, [dirty]);

  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => {
        if (isSet && !replacing && !dirty) {
          return (
            <HStack gap={2} align="center" className="secret-field">
              {isMobile ? (
                // A full-width field reads better on a phone than a short row
                // of dots above a lone button.
                <TextInput label={label} value={savedMasked || "Saved"} isReadOnly isLabelHidden />
              ) : (
                <Text
                  type="supporting"
                  color="secondary"
                  style={{ fontFamily: "var(--font-mono)" }}
                >
                  {savedMasked || "Saved"}
                </Text>
              )}
              <Button
                label="Replace"
                variant="secondary"
                size="sm"
                onClick={() => setReplacing(true)}
                data-cuelume-press
              />
            </HStack>
          );
        }
        return (
          <HStack gap={2} align="center" className="secret-field">
            <TextInput
              label={label}
              type={visible ? "text" : "password"}
              value={field.value}
              onChange={(v) => {
                field.onChange(v);
                onDirtyChange(true);
                onEdit?.();
              }}
              placeholder={isSet ? "Enter new value" : emptyPlaceholder}
              isLabelHidden
              width={width}
            />
            <Button
              label={visible ? "Hide" : "Show"}
              variant="ghost"
              isIconOnly
              icon={visible ? <EyeOff size={16} /> : <Eye size={16} />}
              onClick={() => setVisible((v) => !v)}
              data-cuelume-toggle="whisper"
            />
            {isSet && (
              <Button
                label="Cancel"
                variant="ghost"
                size="sm"
                onClick={() => {
                  field.onChange("");
                  setReplacing(false);
                  setVisible(false);
                  onDirtyChange(false);
                  onEdit?.();
                }}
                data-cuelume-press
              />
            )}
          </HStack>
        );
      }}
    />
  );
}
