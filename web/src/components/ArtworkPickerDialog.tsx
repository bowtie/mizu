import { useEffect, useRef, useState } from "react";
import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TabList, Tab } from "@astryxdesign/core/TabList";
import { SegmentedControl, SegmentedControlItem } from "@astryxdesign/core/SegmentedControl";
import { Divider } from "@astryxdesign/core/Divider";
import { VStack } from "@astryxdesign/core/VStack";
import { Text } from "@astryxdesign/core/Text";
import { HStack } from "@astryxdesign/core/HStack";
import { Button } from "@astryxdesign/core/Button";
import { Selector } from "@astryxdesign/core/Selector";
import { Badge } from "@astryxdesign/core/Badge";
import { RefreshCw, Upload, Trash2 } from "lucide-react";
import { Grid } from "@astryxdesign/core/Grid";
import { ClickableCard } from "@astryxdesign/core/ClickableCard";
import { AspectRatio } from "@astryxdesign/core/AspectRatio";
import { getArtworkCandidates } from "../lib/api";
import type { ArtworkOption, ArtworkOptions, SelectOption } from "../lib/types";

export type ArtworkSlot = "poster" | "banner" | "logo";

export interface ArtworkPickerSource {
  id: string;
  label: string;
}

const SLOT_META: {
  slot: ArtworkSlot;
  key: "posters" | "banners" | "logos";
  label: string;
  emptyLabel: string;
  ratio: number;
  fit: "cover" | "contain";
  minWidth: number;
}[] = [
  {
    slot: "poster",
    key: "posters",
    label: "Posters",
    emptyLabel: "No posters from this source.",
    ratio: 2 / 3,
    fit: "cover",
    minWidth: 96,
  },
  {
    slot: "banner",
    key: "banners",
    label: "Banners",
    emptyLabel: "No banners from this source.",
    ratio: 16 / 9,
    fit: "cover",
    minWidth: 220,
  },
  {
    slot: "logo",
    key: "logos",
    label: "Logos",
    emptyLabel: "No logos from this source.",
    ratio: 3 / 1,
    fit: "contain",
    minWidth: 200,
  },
];

interface ArtworkPickerDialogProps {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  showId: string | number;
  /** Candidate-capable sources to tab between (e.g. fanart, tmdb). */
  sources: ArtworkPickerSource[];
  /** Source to start on; falls back to the first source when unset/invalid. */
  initialSource?: string;
  /** Source URLs currently set on the show, keyed by slot. The tile whose
   *  candidate URL matches gets a selected ring so the active choice is
   *  visible while browsing. */
  currentArtwork?: Partial<Record<ArtworkSlot, string>>;
  onSelect: (source: string, slot: ArtworkSlot, url: string) => void | Promise<void>;
  /** Where this show's artwork comes from ("" = the global default). */
  artworkSource: string;
  artworkSourceOptions: SelectOption[];
  refreshing: boolean;
  onArtworkSourceChange: (source: string) => void;
  onRefresh: () => void;
  /** Slots currently holding an image the user uploaded. */
  customSlots: string[];
  onUploadCustom: (slot: ArtworkSlot, file: File) => void | Promise<void>;
  onRemoveCustom: (slot: ArtworkSlot) => void | Promise<void>;
}

/**
 * Lists a multi-image source's candidate posters/banners/logos and lets the
 * user pick one. Sources are switched via tabs (fanart/tmdb), and the three
 * slots (poster/banner/logo) are shown one at a time via a second tab strip,
 * each rendered with an aspect ratio matching its kind (tall posters, wide
 * banners, wide logos). Picking emits `onSelect(source, slot, url)`; the
 * parent persists the chosen image (partial pick — only that slot changes)
 * and closes the dialog.
 */
export function ArtworkPickerDialog({
  isOpen,
  onOpenChange,
  showId,
  sources,
  initialSource,
  currentArtwork,
  onSelect,
  artworkSource,
  artworkSourceOptions,
  refreshing,
  onArtworkSourceChange,
  onRefresh,
  customSlots,
  onUploadCustom,
  onRemoveCustom,
}: ArtworkPickerDialogProps) {
  const [selectedSource, setSelectedSource] = useState<string | null>(null);
  const [activeSlot, setActiveSlot] = useState<ArtworkSlot>("poster");
  const [options, setOptions] = useState<ArtworkOptions | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Keep already-fetched candidates so switching source tabs is instant and
  // never re-scrapes a provider the user has already browsed.
  const cacheRef = useRef<Record<string, ArtworkOptions>>({});
  // The tile being saved. The dialog stays open after a pick so each slot
  // can be set in turn; this dims the tile until the save lands.
  const [savingUrl, setSavingUrl] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const pick = async (source: string, slot: ArtworkSlot, url: string) => {
    setSavingUrl(url);
    try {
      await onSelect(source, slot, url);
    } finally {
      setSavingUrl(null);
    }
  };

  // On phones the picker renders as a full-screen sheet (variant="fullscreen")
  // so the source/slot tabs and thumbnails get room instead of being squeezed
  // into a 90vw centered box. Mirrors ShowDetailPage's viewport check.
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const updateViewport = () => setIsMobile(window.innerWidth <= 600);
    updateViewport();
    window.addEventListener("resize", updateViewport);
    return () => window.removeEventListener("resize", updateViewport);
  }, []);

  const activeSource =
    selectedSource && sources.some((s) => s.id === selectedSource)
      ? selectedSource
      : initialSource && sources.some((s) => s.id === initialSource)
        ? initialSource
        : (sources[0]?.id ?? "");

  useEffect(() => {
    if (!isOpen || !activeSource) return;
    const cached = cacheRef.current[activeSource];
    if (cached) {
      setOptions(cached);
      setError(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    setOptions(null);
    getArtworkCandidates(showId, activeSource)
      .then((opts) => {
        if (cancelled) return;
        cacheRef.current[activeSource] = opts;
        setOptions(opts);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, activeSource, showId]);

  const activeMeta = SLOT_META.find((m) => m.slot === activeSlot) ?? SLOT_META[0];
  const items = options ? options[activeMeta.key] : [];
  // The show's currently-set artwork for the active slot; the candidate
  // whose URL matches is ringed as the in-use choice.
  const currentUrl = currentArtwork?.[activeSlot];
  const activeSlotIsCustom = customSlots.includes(activeSlot);
  const allEmpty =
    options &&
    options.posters.length === 0 &&
    options.banners.length === 0 &&
    options.logos.length === 0;

  return (
    <Dialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      variant={isMobile ? "fullscreen" : "standard"}
      width={720}
      maxHeight="85vh"
      purpose="info"
    >
      <VStack gap={4} className="artwork-picker-dialog">
        <DialogHeader
          title="Artwork"
          subtitle="Pick an image for each slot. Your choices apply right away."
          onOpenChange={onOpenChange}
        />
        <HStack gap={3} align="center" wrap="wrap" hAlign="between">
          <TabList
            value={activeSlot}
            onChange={(v) => setActiveSlot(v as ArtworkSlot)}
            aria-label="Artwork type"
          >
            {SLOT_META.map((m) => (
              <Tab key={m.slot} value={m.slot} label={m.label} />
            ))}
          </TabList>
          {sources.length > 1 && (
            <SegmentedControl
              label="Image source"
              size="sm"
              value={activeSource}
              onChange={setSelectedSource}
            >
              {sources.map((s) => (
                <SegmentedControlItem key={s.id} value={s.id} label={s.label} />
              ))}
            </SegmentedControl>
          )}
        </HStack>
        <HStack gap={2} align="center" wrap="wrap" className="artwork-picker-custom">
          <Button
            label={`Upload your ${activeSlot}`}
            variant="secondary"
            size="sm"
            icon={<Upload size={14} />}
            isDisabled={refreshing}
            onClick={() => fileInputRef.current?.click()}
            data-cuelume-press
          />
          <input
            ref={fileInputRef}
            type="file"
            hidden
            data-testid="artwork-upload-input"
            accept="image/jpeg,image/png,image/gif,image/webp"
            onChange={(e) => {
              const file = e.target.files?.[0];
              // Reset so picking the same file again still fires onChange.
              e.target.value = "";
              if (file) void onUploadCustom(activeSlot, file);
            }}
          />
          {activeSlotIsCustom ? (
            <>
              <Badge variant="info" label="Your image is in use" />
              <Button
                label="Remove"
                variant="ghost"
                size="sm"
                icon={<Trash2 size={14} />}
                isDisabled={refreshing}
                onClick={() => void onRemoveCustom(activeSlot)}
                data-cuelume-press
              />
            </>
          ) : (
            <Text type="supporting" color="secondary">
              JPG, PNG, GIF or WebP, up to 10 MB
            </Text>
          )}
        </HStack>
        {sources.length === 0 && (
          <Text type="supporting" color="secondary">
            Add a TMDB or Fanart API key in Settings to browse their images.
          </Text>
        )}
        {loading && (
          <Text type="supporting" color="secondary">
            Loading candidates…
          </Text>
        )}
        {error && (
          <Text type="supporting" color="secondary">
            {error}
          </Text>
        )}
        {!loading && !error && options && !allEmpty && (
          <>
            <div className="artwork-picker-sections">
              {items.length > 0 ? (
                <Grid
                  columns={{
                    minWidth: isMobile && activeSlot === "poster" ? 80 : activeMeta.minWidth,
                  }}
                  gap={2}
                >
                  {items.map((opt: ArtworkOption, i: number) => {
                    const meta = [
                      opt.lang ? `language: ${opt.lang}` : "",
                      opt.likes ? `${opt.likes} likes` : "",
                    ]
                      .filter(Boolean)
                      .join(" · ");
                    // A merged duplicate may be the copy the show already uses.
                    const isCurrent =
                      currentUrl !== undefined &&
                      (opt.url === currentUrl || !!opt.alt_urls?.includes(currentUrl));
                    return (
                      <ClickableCard
                        key={opt.url}
                        label={`${activeMeta.label} ${i + 1}${meta ? ` — ${meta}` : ""}`}
                        onClick={() => void pick(activeSource, activeSlot, opt.url)}
                        isDisabled={refreshing}
                        padding={0}
                        className={`artwork-picker-item${isCurrent ? " is-selected" : ""}${savingUrl === opt.url ? " is-saving" : ""}`}
                        data-slot={activeSlot}
                        data-selected={isCurrent || undefined}
                        aria-pressed={isCurrent || undefined}
                      >
                        <AspectRatio ratio={activeMeta.ratio} fit={activeMeta.fit}>
                          <img
                            src={opt.thumb_url || opt.url}
                            alt={`${activeMeta.label} ${i + 1}`}
                            loading="lazy"
                            decoding="async"
                            className={activeSlot === "logo" ? "artwork-picker-logo" : undefined}
                          />
                        </AspectRatio>
                        {isCurrent && (
                          <span className="artwork-picker-check" aria-hidden="true">
                            ✓
                          </span>
                        )}
                      </ClickableCard>
                    );
                  })}
                </Grid>
              ) : (
                <Text type="supporting" color="secondary">
                  {activeMeta.emptyLabel}
                </Text>
              )}
            </div>
          </>
        )}
        {!loading && !error && allEmpty && (
          <Text type="supporting" color="secondary">
            This source has no selectable images.
          </Text>
        )}
        <Divider />
        <HStack gap={3} align="center" wrap="wrap" hAlign="between">
          <HStack gap={2} align="center" wrap="wrap">
            <Text type="supporting" color="secondary">
              Automatic source
            </Text>
            <Selector
              label="Automatic artwork source"
              value={artworkSource}
              options={artworkSourceOptions}
              onChange={(v) => onArtworkSourceChange(v as string)}
              isLabelHidden
              isDisabled={refreshing}
              width={200}
            />
            <Button
              label={refreshing ? "Refreshing..." : "Refresh"}
              variant="secondary"
              size="md"
              icon={<RefreshCw size={14} className={refreshing ? "spin" : ""} />}
              onClick={() => onRefresh()}
              isDisabled={refreshing}
              data-cuelume-press
            />
          </HStack>
          <Button label="Done" variant="primary" size="md" onClick={() => onOpenChange(false)} data-cuelume-press />
        </HStack>
      </VStack>
    </Dialog>
  );
}
