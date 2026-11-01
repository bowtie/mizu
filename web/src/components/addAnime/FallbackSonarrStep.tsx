import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { List, ListItem } from "@astryxdesign/core/List";
import { Search, Check } from "lucide-react";
import type { AddWizardState, AddWizardAction } from "../../routes/addWizardHelpers";

interface FallbackSonarrStepProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  loadSonarrLookupResults: (query?: string) => void | Promise<void>;
  handleAddSelectedSonarrLookup: () => void | Promise<void>;
}

export function FallbackSonarrStep({
  w,
  dispatch,
  loadSonarrLookupResults,
  handleAddSelectedSonarrLookup,
}: FallbackSonarrStepProps) {
  return (
    <VStack gap={4}>
      <TextInput
        label="Sonarr lookup"
        value={w.sonarrLookupQuery}
        onChange={(v) => dispatch({ type: "SET", payload: { sonarrLookupQuery: v } })}
        placeholder="Search Sonarr for series to add"
        startIcon={Search}
      />
      <Button
        label="Search Sonarr"
        variant="secondary"
        onClick={() => loadSonarrLookupResults()}
        isLoading={w.sonarrLookupLoading}
        data-cuelume-press
      />
      {w.sonarrLookupResults.length > 0 && (
        <List hasDividers className="wizard-list">
          {w.sonarrLookupResults.map((series) => {
            const isSelected = w.selectedSonarrLookup?.id === series.id;
            return (
              <ListItem
                key={series.id}
                label={series.title}
                description={String(series.year || "Anime")}
                onClick={() => dispatch({ type: "SET", payload: { selectedSonarrLookup: series } })}
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
          label="Add to Sonarr and match"
          variant="primary"
          onClick={handleAddSelectedSonarrLookup}
          isLoading={w.sonarrAdding}
          isDisabled={!w.selectedSonarrLookup}
          data-cuelume-press
        />
      </HStack>
    </VStack>
  );
}
