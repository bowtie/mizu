import { VStack } from "@astryxdesign/core/VStack";
import { Section } from "@astryxdesign/core/Section";

/** Placeholder shown while settings load. */
export function SettingsSkeleton() {
  return (
    <Section padding={4} maxWidth={900} className="section-centered">
      <VStack gap={6}>
        {/* Skeleton header */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-1)" }}>
            <div className="skeleton-text-lg" style={{ width: 120 }} />
            <div className="skeleton-text" style={{ width: 280 }} />
          </div>
          <div
            className="skeleton"
            style={{ width: 110, height: 36, borderRadius: "var(--radius-inner)" }}
          />
        </div>
        {/* Skeleton connection card */}
        <div
          className="skeleton-card"
          style={{
            padding: "var(--spacing-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--spacing-3)",
          }}
        >
          <div className="skeleton-text-lg" style={{ width: 120 }} />
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-2)" }}>
            <div className="skeleton-text" style={{ width: 80 }} />
            <div
              className="skeleton"
              style={{ width: "100%", height: 36, borderRadius: "var(--radius-inner)" }}
            />
            <div className="skeleton-text" style={{ width: 60 }} />
            <div
              className="skeleton"
              style={{ width: "100%", height: 36, borderRadius: "var(--radius-inner)" }}
            />
          </div>
        </div>
        {/* Skeleton sync card */}
        <div
          className="skeleton-card"
          style={{
            padding: "var(--spacing-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--spacing-3)",
          }}
        >
          <div className="skeleton-text-lg" style={{ width: 80 }} />
          <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
            <div className="skeleton-text" style={{ width: 100 }} />
            <div
              className="skeleton"
              style={{ width: 160, height: 36, borderRadius: "var(--radius-inner)" }}
            />
          </div>
        </div>
        {/* Skeleton notifications card */}
        <div
          className="skeleton-card"
          style={{
            padding: "var(--spacing-4)",
            display: "flex",
            flexDirection: "column",
            gap: "var(--spacing-3)",
          }}
        >
          <div className="skeleton-text-lg" style={{ width: 110 }} />
          <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-2)" }}>
            <div className="skeleton-text" style={{ width: 70 }} />
            <div
              className="skeleton"
              style={{ width: "100%", height: 36, borderRadius: "var(--radius-inner)" }}
            />
          </div>
        </div>
      </VStack>
    </Section>
  );
}
