/** Placeholder shown while the show record is still loading. */
export function ShowDetailSkeleton() {
  return (
    <>
      {/* Skeleton: Artwork card */}
      <div className="skeleton-card" style={{ padding: "var(--spacing-4)" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-3)" }}>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
            <div className="skeleton" style={{ width: 16, height: 16, borderRadius: 4 }} />
            <div className="skeleton-text" style={{ width: 80 }} />
            <div style={{ flex: 1 }} />
            <div
              className="skeleton"
              style={{ width: 90, height: 28, borderRadius: "var(--radius-inner)" }}
            />
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
            <div className="skeleton-text" style={{ width: 60 }} />
            <div
              className="skeleton"
              style={{ width: 220, height: 32, borderRadius: "var(--radius-inner)" }}
            />
          </div>
        </div>
      </div>

      {/* Skeleton: Tracking rules */}
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--spacing-4)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: "var(--spacing-3)" }}>
          <div className="skeleton-text-lg" style={{ width: 130 }} />
          <div style={{ flex: 1 }} />
        </div>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
            gap: "var(--spacing-4)",
          }}
        >
          <div className="skeleton-rule-card">
            <div className="skeleton-text" style={{ width: 60 }} />
            <div className="skeleton-text-lg" style={{ width: 100 }} />
            <div className="skeleton-text" style={{ width: "80%" }} />
            <div style={{ flex: 1 }} />
            <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
          </div>
          <div className="skeleton-rule-card">
            <div className="skeleton-text" style={{ width: 50 }} />
            <div className="skeleton-text-lg" style={{ width: 90 }} />
            <div className="skeleton-text" style={{ width: "70%" }} />
            <div style={{ flex: 1 }} />
            <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
          </div>
          <div className="skeleton-rule-card">
            <div className="skeleton-text" style={{ width: 45 }} />
            <div className="skeleton-text-lg" style={{ width: 80 }} />
            <div className="skeleton-text" style={{ width: "75%" }} />
            <div style={{ flex: 1 }} />
            <div className="skeleton" style={{ width: 44, height: 24, borderRadius: 12 }} />
          </div>
        </div>
      </div>
    </>
  );
}
