import { HStack } from "@astryxdesign/core/HStack";
import { Button } from "@astryxdesign/core/Button";
import { NumberInput } from "@astryxdesign/core/NumberInput";
import { InputGroup, InputGroupText } from "@astryxdesign/core/InputGroup";
import { DropdownMenu } from "@astryxdesign/core/DropdownMenu";
import {
  Trash2,
  Bot,
  Minus,
  Plus,
  Bell,
  BellOff,
  Palette,
  HardDrive,
  MoreHorizontal,
} from "lucide-react";

interface ShowActionsBarProps {
  watermarkAuto: boolean;
  markerValue: number;
  maxEpisode: number;
  isMobile: boolean;
  deleting: boolean;
  notificationsMuted: boolean;
  notificationsSaving: boolean;
  /** Receives the requested marker; the parent clamps it. */
  onMarkerChange: (value: number) => void;
  onToggleAuto: () => void;
  onToggleMuted: () => void;
  onEditArtwork: () => void;
  /** Whether the show has filler files to delete; the action shows only then. */
  hasFillerFiles: boolean;
  onFillerFiles: () => void;
  onRemove: () => void;
}

/** Watermark stepper and the show's actions, revealed once show data is in.
 *  Changes save themselves (see ShowDetailPage), so there is no Save button.
 *  Phones get one row: the stepper, Auto, and the rest in a ⋯ menu. */
export function ShowActionsBar({
  watermarkAuto,
  markerValue,
  maxEpisode,
  isMobile,
  deleting,
  notificationsMuted,
  notificationsSaving,
  onMarkerChange,
  onToggleAuto,
  onToggleMuted,
  onEditArtwork,
  hasFillerFiles,
  onFillerFiles,
  onRemove,
}: ShowActionsBarProps) {
  const stepper = (
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
  );

  const autoToggle = (
    <Button
      label={watermarkAuto ? "Auto" : "Manual"}
      variant="secondary"
      size="md"
      icon={<Bot size={16} />}
      onClick={() => onToggleAuto()}
      aria-label={watermarkAuto ? "Using Sonarr progress" : "Using manual episode"}
      data-cuelume-toggle
    />
  );

  if (isMobile) {
    return (
      <div className="controls-reveal">
        <div className="controls-reveal-inner">
          <div className="show-actions-bar show-actions-bar-mobile">
            {stepper}
            {autoToggle}
            <DropdownMenu
              button={{
                label: "More actions",
                variant: "secondary",
                size: "md",
                isIconOnly: true,
                icon: <MoreHorizontal size={16} />,
              }}
              items={[
                { label: "Artwork", icon: <Palette size={16} />, onClick: onEditArtwork },
                {
                  label: notificationsMuted ? "Unmute notifications" : "Mute notifications",
                  icon: notificationsMuted ? <Bell size={16} /> : <BellOff size={16} />,
                  isDisabled: notificationsSaving,
                  onClick: onToggleMuted,
                },
                ...(hasFillerFiles
                  ? [
                      {
                        label: "Delete filler files",
                        icon: <HardDrive size={16} />,
                        onClick: onFillerFiles,
                      },
                    ]
                  : []),
                { type: "divider" as const },
                {
                  label: "Remove from Mizu",
                  icon: <Trash2 size={16} />,
                  variant: "destructive" as const,
                  isDisabled: deleting,
                  onClick: onRemove,
                },
              ]}
            />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="controls-reveal">
      <div className="controls-reveal-inner">
        <div className="show-actions-bar">
          <HStack gap={3} align="center" wrap="nowrap">
            {stepper}
            {autoToggle}
            <Button
              label="Edit artwork"
              tooltip="Artwork"
              variant="secondary"
              size="md"
              isIconOnly
              icon={<Palette size={16} />}
              onClick={() => onEditArtwork()}
              data-cuelume-press
            />
            <Button
              label={notificationsMuted ? "Notifications muted" : "Notifications on"}
              variant="secondary"
              size="md"
              isIconOnly
              icon={notificationsMuted ? <BellOff size={16} /> : <Bell size={16} />}
              tooltip={notificationsMuted ? "Click to unmute" : "Click to mute"}
              isDisabled={notificationsSaving}
              onClick={() => onToggleMuted()}
              data-cuelume-toggle
            />
          </HStack>
          <HStack gap={2} align="center" wrap="nowrap" className="show-actions-bar-end">
            {hasFillerFiles && (
              <Button
                label="Delete filler files"
                tooltip="Delete filler files"
                variant="secondary"
                size="md"
                isIconOnly
                icon={<HardDrive size={16} />}
                onClick={() => onFillerFiles()}
                data-cuelume-press
              />
            )}
            <Button
              label="Remove"
              variant="destructive"
              size="md"
              icon={<Trash2 size={16} />}
              onClick={() => onRemove()}
              isDisabled={deleting}
              data-cuelume-press
            />
          </HStack>
        </div>
      </div>
    </div>
  );
}
