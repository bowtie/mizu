import { VStack } from "@astryxdesign/core/VStack";
import { HStack } from "@astryxdesign/core/HStack";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { ChevronDown } from "lucide-react";
import type { Show } from "../lib/types";

interface ShowHeroProps {
  show: Show | null;
  /** Backdrop URL, or empty when nothing should be rendered. */
  backdropUrl: string;
  /** Changes whenever the image should be re-requested. */
  imageKey: string;
  onImageError: () => void;
  /** Set on touch screens, where tapping the hero shows or hides the
   *  actions bar that slides out on hover elsewhere. */
  controlsToggle?: { isOpen: boolean; onToggle: () => void };
}

/** Hero header with backdrop — renders immediately using the showId from the
 *  URL. The image starts loading before getShow() resolves. */
export function ShowHero({
  show,
  backdropUrl,
  imageKey,
  onImageError,
  controlsToggle,
}: ShowHeroProps) {
  const toggleProps = controlsToggle && {
    role: "button",
    tabIndex: 0,
    "aria-expanded": controlsToggle.isOpen,
    "aria-label": controlsToggle.isOpen ? "Hide show actions" : "Show show actions",
    onClick: controlsToggle.onToggle,
    onKeyDown: (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        controlsToggle.onToggle();
      }
    },
  };
  return (
    <div className="show-hero" {...toggleProps}>
      {backdropUrl ? (
        <>
          <img
            key={imageKey}
            src={backdropUrl}
            alt=""
            loading="eager"
            decoding="async"
            fetchPriority="high"
            className="show-hero-img"
            onError={onImageError}
          />
        </>
      ) : (
        <div className="show-hero-placeholder" />
      )}
      <div className="show-hero-content">
        <VStack gap={1} className="show-hero-titles" style={{ padding: 0, margin: 0 }}>
          <HStack
            gap={3}
            align="center"
            wrap="wrap"
            style={{ padding: 0, margin: 0, alignItems: "center" }}
          >
            {show ? (
              <Heading level={1} className="show-hero-title">
                {show.sonarr_title}
              </Heading>
            ) : (
              <div className="skeleton-text-lg" style={{ width: 220, height: "1.5em" }} />
            )}
          </HStack>
          {show?.overview && (
            <Text
              type="supporting"
              color="secondary"
              display="block"
              className="show-hero-overview"
              style={{ margin: 0 }}
            >
              {show.overview}
            </Text>
          )}
        </VStack>
      </div>
      {controlsToggle && (
        <span
          className={`show-hero-toggle${controlsToggle.isOpen ? " is-open" : ""}`}
          aria-hidden="true"
        >
          <ChevronDown size={18} />
        </span>
      )}
    </div>
  );
}
