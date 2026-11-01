import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Button } from "@astryxdesign/core/Button";
import { Spinner } from "@astryxdesign/core/Spinner";
import { EmptyState } from "@astryxdesign/core/EmptyState";
import { Icon } from "@astryxdesign/core/Icon";
import { TextInput } from "@astryxdesign/core/TextInput";
import { List, ListItem } from "@astryxdesign/core/List";
import { Search, Check } from "lucide-react";
import type { AddWizardState, AddWizardAction } from "../../routes/addWizardHelpers";

interface FallbackAflStepProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  scheduleFallbackAflFilter: () => void;
  goToFallbackSonarrStep: () => void | Promise<void>;
}

export function FallbackAflStep({
  w,
  dispatch,
  scheduleFallbackAflFilter,
  goToFallbackSonarrStep,
}: FallbackAflStepProps) {
  return (
    <VStack gap={4}>
      <TextInput
        label="Cached AFL title"
        isLabelHidden
        startIcon={Search}
        value={w.fallbackAflQuery}
        onChange={(v) => {
          dispatch({ type: "SET", payload: { fallbackAflQuery: v } });
          scheduleFallbackAflFilter();
        }}
        placeholder="Filter cached AnimeFillerList titles"
      />
      {w.fallbackAflLoading ? (
        <VStack gap={3} hAlign="center" padding={6}>
          <Spinner size="lg" label="Loading cached AnimeFillerList titles" />
        </VStack>
      ) : w.fallbackAflResults.length === 0 ? (
        <EmptyState
          title="No cached AFL titles found"
          description="Try another filter or refresh the AFL cache in Settings if this anime is new."
          icon={<Icon icon={Search} size="lg" color="secondary" />}
        />
      ) : (
        <List hasDividers className="wizard-list">
          {w.fallbackAflResults.map((result) => {
            const isSelected = w.selectedFallbackAfl?.slug === result.slug;
            return (
              <ListItem
                key={result.slug}
                label={result.title || ""}
                onClick={() => dispatch({ type: "SET", payload: { selectedFallbackAfl: result } })}
                endContent={
                  isSelected ? <Check size={16} color="var(--color-accent)" /> : undefined
                }
              />
            );
          })}
        </List>
      )}
      <HStack justify="end">
        <Button
          label="Continue to Sonarr lookup"
          variant="primary"
          onClick={goToFallbackSonarrStep}
          isDisabled={!w.selectedFallbackAfl}
          data-cuelume-press
        />
      </HStack>
    </VStack>
  );
}
