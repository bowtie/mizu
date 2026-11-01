import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Check } from "lucide-react";
import type { AddWizardState, AddWizardAction } from "../../routes/addWizardHelpers";

interface PostAddStepProps {
  w: AddWizardState;
  dispatch: React.Dispatch<AddWizardAction>;
  openShow: (id: string | number) => void;
  returnToSeriesStep: (prefill?: string) => void;
}

export function PostAddStep({ w, dispatch, openShow, returnToSeriesStep }: PostAddStepProps) {
  return (
    <VStack gap={4} hAlign="center" padding={6} className="wizard-success">
      <div className="wizard-success-icon">
        <Check size={32} color="var(--color-success)" />
      </div>
      <Heading level={3}>{w.addOutcome?.title}</Heading>
      <Text type="supporting">
        {w.addOutcome?.type === "mapping_created"
          ? "Now tracked by Mizu."
          : "In Sonarr now, but still needs a safe AFL mapping before Mizu can track it."}
      </Text>
      <HStack gap={2}>
        {w.addOutcome?.mappingId && (
          <Button
            label="Open anime"
            variant="primary"
            onClick={() => {
              dispatch({ type: "SET", payload: { showAddModal: false } });
              openShow(w.addOutcome!.mappingId!);
            }}
            data-cuelume-press
          />
        )}
        <Button
          label="Add another"
          variant="secondary"
          onClick={() => returnToSeriesStep()}
          data-cuelume-press
        />
      </HStack>
    </VStack>
  );
}
