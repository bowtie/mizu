import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Text } from "@astryxdesign/core/Text";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { CheckboxInput } from "@astryxdesign/core/CheckboxInput";
import { Trash2 } from "lucide-react";
import { useNotificationStore } from "../lib/store";
import { queryKeys } from "../lib/queries";
import { deleteFillerFiles, errorMessage, getFillerFiles, type FillerFile } from "../lib/api";

/** Binary units, matching the activity log. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let value = n / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

function describe(file: FillerFile): string {
  const nums = file.episodes.map((ep) => ep.episode_num).join(", ");
  const title = file.episodes.length === 1 ? ` · ${file.episodes[0].title}` : "";
  return `Episode ${nums}${title}`;
}

interface FillerFilesDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  showId: string;
  isMobile: boolean;
}

/** Lists downloaded files that only hold episodes the show's rules skip,
 *  and deletes the picked ones through Sonarr. */
export function FillerFilesDialog({
  isOpen,
  onOpenChange,
  showId,
  isMobile,
}: FillerFilesDialogProps) {
  const queryClient = useQueryClient();
  const addNotification = useNotificationStore((s) => s.addNotification);
  const query = useQuery({
    queryKey: ["fillerFiles", showId],
    queryFn: () => getFillerFiles(showId),
    enabled: isOpen,
    staleTime: 0,
  });
  const files = useMemo(() => query.data?.files ?? [], [query.data]);
  const [picked, setPicked] = useState<Set<number>>(new Set());
  const [deleting, setDeleting] = useState(false);

  // Start with every file picked except ones monitored again on purpose.
  useEffect(() => {
    setPicked(new Set(files.filter((f) => !f.monitored).map((f) => f.episode_file_id)));
  }, [files]);

  const pickedSize = files
    .filter((f) => picked.has(f.episode_file_id))
    .reduce((sum, f) => sum + f.size, 0);

  const toggle = (id: number, on: boolean) =>
    setPicked((prev) => {
      const next = new Set(prev);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });

  const remove = async () => {
    setDeleting(true);
    try {
      const res = await deleteFillerFiles(showId, [...picked]);
      if (res.error) {
        addNotification(
          `Deleted ${res.deleted} file${res.deleted === 1 ? "" : "s"}, then failed: ${res.error}`,
          "error",
        );
      } else {
        addNotification(
          `Deleted ${res.deleted} file${res.deleted === 1 ? "" : "s"}, freed ${formatBytes(res.freed_bytes)}`,
          "success",
        );
        onOpenChange(false);
      }
    } catch (err) {
      addNotification(`Couldn't delete files: ${errorMessage(err)}`, "error");
    } finally {
      setDeleting(false);
      void queryClient.invalidateQueries({ queryKey: ["fillerFiles", showId] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.episodes(showId) });
      void queryClient.invalidateQueries({ queryKey: ["syncLogs"] });
    }
  };

  let body;
  if (query.isPending) {
    body = <Text color="secondary">Looking through Sonarr's files…</Text>;
  } else if (query.error) {
    body = <Text color="secondary">Couldn't list files: {errorMessage(query.error)}</Text>;
  } else if (files.length === 0) {
    body = <Text color="secondary">No downloaded files hold only skipped episodes.</Text>;
  } else {
    body = (
      <VStack gap={2}>
        {files.map((f) => (
          <HStack key={f.episode_file_id} gap={2} vAlign="center" hAlign="between" wrap="wrap">
            <CheckboxInput
              label={describe(f)}
              description={`${f.path} · ${formatBytes(f.size)}`}
              value={picked.has(f.episode_file_id)}
              onChange={(on) => toggle(f.episode_file_id, on)}
              isDisabled={deleting}
            />
            {f.monitored && <Badge label="Monitored" />}
          </HStack>
        ))}
      </VStack>
    );
  }

  return (
    <Dialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      variant={isMobile ? "fullscreen" : "standard"}
      width={640}
      maxHeight="85vh"
      purpose="info"
    >
      <VStack gap={4}>
        <DialogHeader
          title="Delete filler files"
          subtitle="Files holding only episodes this show skips. Sonarr deletes them from disk and they stay unmonitored. This can't be undone."
          onOpenChange={onOpenChange}
        />
        {body}
        <HStack gap={2} hAlign="end">
          <Button label="Cancel" variant="ghost" onClick={() => onOpenChange(false)} />
          <Button
            label={
              picked.size > 0
                ? `Delete ${picked.size} file${picked.size === 1 ? "" : "s"} (${formatBytes(pickedSize)})`
                : "Delete"
            }
            variant="destructive"
            icon={<Trash2 size={16} />}
            isLoading={deleting}
            isDisabled={deleting || picked.size === 0}
            onClick={() => void remove()}
          />
        </HStack>
      </VStack>
    </Dialog>
  );
}
