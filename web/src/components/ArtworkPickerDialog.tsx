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
import { Skeleton } from "@astryxdesign/core/Skeleton";
import { Spinner } from "@astryxdesign/core/Spinner";
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
    minWidth: 140,
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

/** Thumbnail that shows a skeleton until the image has loaded, then fades in. */
function ArtworkThumb({
  src,
  alt,
  index,
  className,
}: {
  src: string;
  alt: string;
  index: number;
  className?: string;
}) {
  const [loaded, setLoaded] = useState(false);
  const imgRef = useRef<HTMLImageElement | null>(null);
  // A cached image can finish before React attaches onLoad.
  useEffect(() => {
    if (imgRef.current?.complete && imgRef.current.naturalWidth > 0) setLoaded(true);
  }, []);
  return (
    <>
      {!loaded && (
        <span className="artwork-picker-thumb-skeleton" aria-hidden="true">
          <Skeleton index={index % 12} />
        </span>
      )}
      <img
        ref={imgRef}
        src={src}
        alt={alt}
        loading="lazy"
        decoding="async"
        onLoad={() => setLoaded(true)}
        // Show the broken-image state rather than a skeleton forever.
        onError={() => setLoaded(true)}
        className={`artwork-picker-thumb${loaded ? " is-loaded" : ""}${className ? ` ${className}` : ""}`}
      />
    </>
  );
}

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
  const [uploading, setUploading] = useState(false);

  const upload = async (slot: ArtworkSlot, file: File) => {
    setUploading(true);
    try {
      await onUploadCustom(slot, file);
    } finally {
      setUploading(false);
    }
  };

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
      width={800}
      maxHeight="85vh"
      purpose="info"
    >
      <VStack gap={4} className="artwork-picker-dialog">
        <DialogHeader
          title="Artwork"
          subtitle="Pick an image for each slot. Changes save right away."
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
              size="md"
              value={activeSource}
              onChange={setSelectedSource}
            >
              {sources.map((s) => (
                <SegmentedControlItem key={s.id} value={s.id} label={s.label} />
              ))}
            </SegmentedControl>
          )}
        </HStack>
        {sources.length === 0 && (
          <Text type="supporting" color="secondary">
            Add a TMDB or Fanart key in Settings to use them.
          </Text>
        )}
        {loading && (
          <div
            className="artwork-picker-sections"
            role="status"
            aria-busy="true"
            aria-label="Loading candidates"
          >
            <Grid
              columns={{
                minWidth: isMobile && activeSlot === "poster" ? 100 : activeMeta.minWidth,
              }}
              gap={2}
            >
              {Array.from({ length: activeSlot === "poster" ? 10 : 6 }, (_, i) => (
                <AspectRatio key={i} ratio={activeMeta.ratio}>
                  <Skeleton index={i} />
                </AspectRatio>
              ))}
            </Grid>
          </div>
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
                    minWidth: isMobile && activeSlot === "poster" ? 100 : activeMeta.minWidth,
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
                        label={`${activeMeta.label} ${i + 1}${meta ? ` (${meta})` : ""}`}
                        onClick={() => void pick(activeSource, activeSlot, opt.url)}
                        isDisabled={refreshing}
                        padding={0}
                        className={`artwork-picker-item${isCurrent ? " is-selected" : ""}${savingUrl === opt.url ? " is-saving" : ""}`}
                        data-slot={activeSlot}
                        data-selected={isCurrent || undefined}
                        aria-pressed={isCurrent || undefined}
                      >
                        <AspectRatio ratio={activeMeta.ratio} fit={activeMeta.fit}>
                          <ArtworkThumb
                            src={opt.thumb_url || opt.url}
                            alt={`${activeMeta.label} ${i + 1}`}
                            index={i}
                            className={activeSlot === "logo" ? "artwork-picker-logo" : undefined}
                          />
                        </AspectRatio>
                        {savingUrl === opt.url && (
                          <span className="artwork-picker-saving">
                            <Spinner size="lg" aria-label="Saving" />
                          </span>
                        )}
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
            This source has no images.
          </Text>
        )}
        <Divider />
        <HStack gap={3} align="center" wrap="wrap" hAlign="between">
          <HStack gap={2} align="center" wrap="wrap">
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
              icon={<RefreshCw size={14} />}
              onClick={() => onRefresh()}
              isLoading={refreshing}
              data-cuelume-press
            />
          </HStack>
          <HStack gap={2} align="center" wrap="wrap">
            {activeSlotIsCustom && (
              <>
                <Badge variant="info" label="Your image is in use" />
                <Button
                  label="Remove"
                  variant="ghost"
                  size="md"
                  icon={<Trash2 size={14} />}
                  isDisabled={refreshing || uploading}
                  onClick={() => void onRemoveCustom(activeSlot)}
                  data-cuelume-press
                />
              </>
            )}
            <Button
              label={`Upload ${activeSlot}`}
              tooltip="JPG, PNG, GIF or WebP, up to 10 MB"
              variant="secondary"
              size="md"
              icon={<Upload size={14} />}
              isLoading={uploading}
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
                if (file) void upload(activeSlot, file);
              }}
            />
            <Button
              label="Done"
              variant="primary"
              size="md"
              onClick={() => onOpenChange(false)}
              data-cuelume-press
            />
          </HStack>
        </HStack>
      </VStack>
    </Dialog>
  );
}
