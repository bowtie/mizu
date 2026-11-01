import { HStack } from "@astryxdesign/core/HStack";
import { Button } from "@astryxdesign/core/Button";
import { NumberInput } from "@astryxdesign/core/NumberInput";
import { InputGroup, InputGroupText } from "@astryxdesign/core/InputGroup";
import { Trash2, Save, Check, Bot, Minus, Plus } from "lucide-react";

interface ShowActionsBarProps {
  watermarkAuto: boolean;
  markerValue: number;
  maxEpisode: number;
  isMobile: boolean;
  deleting: boolean;
  saving: boolean;
  savedFlash: boolean;
  rulesDirty: boolean;
  /** Receives the requested marker; the parent clamps it. */
  onMarkerChange: (value: number) => void;
  onToggleAuto: () => void;
  onRemove: () => void;
  onSave: () => void;
}

/** Watermark stepper plus Remove / Save actions, revealed once show data is in. */
export function ShowActionsBar({
  watermarkAuto,
  markerValue,
  maxEpisode,
  isMobile,
  deleting,
  saving,
  savedFlash,
  rulesDirty,
  onMarkerChange,
  onToggleAuto,
  onRemove,
  onSave,
}: ShowActionsBarProps) {
  return (
    <div className="controls-reveal">
      <div className="controls-reveal-inner">
        <div className="show-actions-bar">
          <HStack gap={3} align="center" wrap="nowrap">
            <InputGroup
              label="Episode"
              isLabelHidden
              size="md"
              isDisabled={watermarkAuto}
              className="show-actions-bar-center"
            >
              <InputGroupText className="episode-step-btn-wrap">
                <Button
                  label="Decrease episode"
                  variant="ghost"
                  size="md"
                  isIconOnly
                  icon={<Minus size={16} />}
                  className="episode-step-btn"
                  isDisabled={watermarkAuto || markerValue <= 0}
                  onClick={() => onMarkerChange(markerValue - 1)}
                  data-cuelume-press
                />
              </InputGroupText>
              <NumberInput
                label="Episode"
                isLabelHidden
                value={markerValue}
                onChange={(value) => onMarkerChange(value)}
                min={0}
                max={maxEpisode || null}
                step={1}
                isIntegerOnly
                isDisabled={watermarkAuto}
              />
              <InputGroupText className="episode-step-btn-wrap">
                <Button
                  label="Increase episode"
                  variant="ghost"
                  size="md"
                  isIconOnly
                  icon={<Plus size={16} />}
                  className="episode-step-btn"
                  isDisabled={watermarkAuto}
                  onClick={() => onMarkerChange(markerValue + 1)}
                  data-cuelume-press
                />
              </InputGroupText>
            </InputGroup>
            <Button
              label={watermarkAuto ? "Auto" : "Manual"}
              variant="secondary"
              size="md"
              icon={<Bot size={16} />}
              isIconOnly={isMobile}
              onClick={() => onToggleAuto()}
              aria-label={watermarkAuto ? "Using Sonarr progress" : "Using manual episode"}
              data-cuelume-toggle
            />
          </HStack>
          <HStack gap={2} align="center" wrap="nowrap" className="show-actions-bar-end">
            <Button
              label="Remove"
              variant="destructive"
              size="md"
              icon={<Trash2 size={16} />}
              isIconOnly={isMobile}
              onClick={() => onRemove()}
              isDisabled={deleting}
              data-cuelume-press
            />
            <Button
              label={savedFlash ? "Saved" : saving ? "Saving..." : "Save"}
              variant="primary"
              size="md"
              icon={
                savedFlash ? <Check size={14} color="var(--color-success)" /> : <Save size={14} />
              }
              isIconOnly={isMobile}
              onClick={onSave}
              isDisabled={saving || !rulesDirty}
              isLoading={saving}
              className={savedFlash ? "save-flash" : undefined}
              data-cuelume-press
            />
          </HStack>
        </div>
      </div>
    </div>
  );
}
