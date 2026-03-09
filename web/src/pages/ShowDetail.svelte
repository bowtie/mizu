<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { addNotification } from '../lib/stores.js';
  import { getShows, getSonarrSeries, updateShow, deleteShow, getEpisodes, getSonarrEpisodes, resyncAll, setEpisodeMonitor } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import Modal from '../components/Modal.svelte';
  import IconArrowLeftRegular from 'phosphor-icons-svelte/IconArrowLeftRegular.svelte';
  import IconArrowsClockwiseRegular from 'phosphor-icons-svelte/IconArrowsClockwiseRegular.svelte';
  import IconFloppyDiskRegular from 'phosphor-icons-svelte/IconFloppyDiskRegular.svelte';
  import IconEyeRegular from 'phosphor-icons-svelte/IconEyeRegular.svelte';
  import IconEyeClosedRegular from 'phosphor-icons-svelte/IconEyeClosedRegular.svelte';
  import IconPauseRegular from 'phosphor-icons-svelte/IconPauseRegular.svelte';
  import IconPlayRegular from 'phosphor-icons-svelte/IconPlayRegular.svelte';
  import IconTrashRegular from 'phosphor-icons-svelte/IconTrashRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';

  export let params = {};

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';

  let loading = true;
  let saving = false;
  let deleting = false;
  let resyncingAll = false;
  let togglingSync = false;
  let showDeleteModal = false;
  let show = null;
  let series = null;
  let episodes = [];
  let sonarrSeasons = {};
  let trackFiller = false;
  let trackMixed = true;
  let trackAnimeCanon = true;
  let watermarkMode = WATERMARK_AUTO;
  let markerValue = 0;
  let epFilter = 'all';
  let collapsedSeasons = {};
  let syncingEps = {};

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  $: showId = params.id;
  $: pageTitle = show ? `Mizu - ${show.sonarr_title}` : 'Mizu - Anime';
  $: filteredEpisodes = episodes.filter((ep) => (epFilter === 'all' ? true : ep.type === epFilter));
  $: sonarrByAbsNum = (() => {
    const map = {};
    for (const seasonNum of Object.keys(sonarrSeasons)) {
      for (const ep of sonarrSeasons[seasonNum] || []) {
        if (ep.absoluteEpisodeNumber != null) {
          map[ep.absoluteEpisodeNumber] = ep;
        }
      }
    }
    return map;
  })();
  $: groupedBySeason = (() => {
    const groups = {};
    for (const ep of filteredEpisodes) {
      const sonarrEp = sonarrByAbsNum[ep.episode_num];
      const season = sonarrEp ? sonarrEp.seasonNumber : -1;
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    return Object.keys(groups)
      .map(Number)
      .sort((a, b) => {
        if (a === -1) return 1;
        if (b === -1) return -1;
        if (a === 0 && b > 0) return 1;
        if (b === 0 && a > 0) return -1;
        return b - a;
      })
      .map((season) => ({ season, episodes: groups[season] }));
  })();
  $: epStats = {
    total: episodes.length,
    canon: episodes.filter((ep) => ep.type === 'canon').length,
    mixed: episodes.filter((ep) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep) => ep.type === 'filler').length,
  };
  $: markerSummary = markerValue > 0 ? `Episode #${markerValue}` : 'Episode #0';
  $: confidenceLabel = `${Math.round((show?.match_confidence || 0) * 100)}%`;
  $: subtitle = show ? `Matched to ${show.afl_title || show.afl_slug} with ${confidenceLabel} confidence` : '';

  onMount(async () => {
    await loadShow();
  });

  async function loadShow() {
    loading = true;
    try {
      const [allShows, allSeries, episodesResult, sonarrResult] = await Promise.all([
        getShows(),
        getSonarrSeries(),
        getEpisodes(showId),
        getSonarrEpisodes(showId),
      ]);

      show = (allShows || []).find((item) => String(item.id) === String(showId));
      if (!show) {
        addNotification('Anime not found', 'error');
        push('/shows');
        return;
      }

      series = (allSeries || []).find((item) => item.id === show.sonarr_series_id) || null;
      episodes = episodesResult || [];
      sonarrSeasons = sonarrResult || {};
      trackFiller = actionToTracked(show.filler_action, false);
      trackMixed = actionToTracked(show.mixed_action, true);
      trackAnimeCanon = actionToTracked(show.anime_canon_action, true);
      watermarkMode = show.watermark_mode || WATERMARK_AUTO;
      markerValue = Math.max(0, show.sync_watermark || 0);
    } catch (err) {
      addNotification('Failed to load anime: ' + err.message, 'error');
    }
    loading = false;
  }

  function posterUrl() {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'poster') || series.images.find((image) => image.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    if (preferred.remoteUrl) return preferred.remoteUrl;
    if (preferred.url && /^https?:/i.test(preferred.url)) return preferred.url;
    return '';
  }

  async function handleSave() {
    saving = true;
    try {
      const payload = {
        filler_action: trackedToAction(trackFiller),
        mixed_action: trackedToAction(trackMixed),
        anime_canon_action: trackedToAction(trackAnimeCanon),
        watermark_mode: watermarkMode,
      };
      if (watermarkMode === WATERMARK_MANUAL) {
        payload.sync_watermark = Math.max(0, Number(markerValue) || 0);
      }
      await updateShow(showId, payload);
      addNotification('Anime rules updated', 'success');
      await loadShow();
    } catch (err) {
      addNotification('Failed to save anime: ' + err.message, 'error');
    }
    saving = false;
  }

  async function handleResyncAll() {
    resyncingAll = true;
    try {
      await resyncAll(showId);
      addNotification('Full resync triggered', 'success');
      await loadShow();
    } catch (err) {
      addNotification('Resync failed: ' + err.message, 'error');
    }
    resyncingAll = false;
  }

  async function handleToggleSync() {
    if (!show) return;
    togglingSync = true;
    try {
      await updateShow(showId, { enabled: !show.enabled });
      addNotification(show.enabled ? 'Sync disabled' : 'Sync resumed', 'success');
      await loadShow();
    } catch (err) {
      addNotification('Failed to update sync state: ' + err.message, 'error');
    }
    togglingSync = false;
  }

  async function handleDelete() {
    deleting = true;
    try {
      await deleteShow(showId);
      addNotification('Anime removed', 'success');
      showDeleteModal = false;
      push('/shows');
    } catch (err) {
      addNotification('Failed to remove anime: ' + err.message, 'error');
    }
    deleting = false;
  }

  async function toggleEpisodeMonitor(ep) {
    const sonarrEp = sonarrByAbsNum[ep.episode_num];
    if (!sonarrEp) {
      addNotification('No Sonarr episode found for #' + ep.episode_num, 'error');
      return;
    }
    syncingEps[sonarrEp.id] = true;
    syncingEps = syncingEps;
    try {
      const nextState = !sonarrEp.monitored;
      await setEpisodeMonitor(showId, sonarrEp.id, nextState);
      sonarrEp.monitored = nextState;
      sonarrSeasons = sonarrSeasons;
      addNotification(`Episode #${ep.episode_num} ${nextState ? 'monitored' : 'unmonitored'}`, 'success');
    } catch (err) {
      addNotification('Failed to update episode: ' + err.message, 'error');
    }
    delete syncingEps[sonarrEp.id];
    syncingEps = syncingEps;
  }

  function toggleSeason(seasonNum) {
    collapsedSeasons[seasonNum] = !collapsedSeasons[seasonNum];
    collapsedSeasons = collapsedSeasons;
  }

  function seasonLabel(num) {
    if (num === -1) return 'Unmatched';
    if (num === 0) return 'Specials';
    return `Season ${num}`;
  }

  function epRowClass(type) {
    switch (type) {
      case 'canon': return 'ep-canon';
      case 'mixed_canon': return 'ep-mixed';
      case 'anime_canon': return 'ep-anime-canon';
      case 'filler': return 'ep-filler';
      default: return '';
    }
  }
</script>

<svelte:head>
  <title>{pageTitle}</title>
</svelte:head>

{#if loading}
  <Spinner center size="lg" message="Loading anime detail..." />
{:else if !show}
  <div class="card detail-empty">
    <div class="empty-state compact-empty">
      <div class="empty-icon"><IconWaveSineRegular size={28} weight="regular" /></div>
      <h3>Anime not found</h3>
      <p>The requested sync entry could not be found.</p>
      <a href="#/shows" class="btn btn-primary">Back to Mizu</a>
    </div>
  </div>
{:else}
  <div class="detail-page-shell">
    <section class="detail-header">
      <a class="detail-backlink" href="#/shows"><IconArrowLeftRegular size={16} weight="regular" /> Back to synced anime</a>
      <div class="detail-actions detail-actions-wide">
        <button class="btn btn-danger btn-inline-icon" on:click={() => (showDeleteModal = true)}>
          <IconTrashRegular size={17} weight="regular" />
          <span>Remove Anime</span>
        </button>
        <div class="detail-actions-cluster">
          <button class="btn btn-secondary btn-inline-icon" on:click={handleToggleSync} disabled={togglingSync}>
            {#if show.enabled}
              <IconPauseRegular size={17} weight="regular" />
              <span>{togglingSync ? 'Stopping...' : 'Stop syncing'}</span>
            {:else}
              <IconPlayRegular size={17} weight="regular" />
              <span>{togglingSync ? 'Resuming...' : 'Resume syncing'}</span>
            {/if}
          </button>
          <button class="btn btn-secondary btn-inline-icon" on:click={handleResyncAll} disabled={resyncingAll}>
            <IconArrowsClockwiseRegular size={18} weight="regular" />
            <span>{resyncingAll ? 'Resyncing...' : 'Resync'}</span>
          </button>
          <button class="btn btn-primary btn-inline-icon" on:click={handleSave} disabled={saving}>
            <IconFloppyDiskRegular size={18} weight="regular" />
            <span>{saving ? 'Saving...' : 'Save'}</span>
          </button>
        </div>
      </div>
    </section>

    <section class="card detail-top-card">
      <div class="detail-summary-media" class:no-art={!posterUrl()}>
        {#if posterUrl()}
          <img src={posterUrl()} alt={show.sonarr_title} />
        {:else}
          <div class="detail-fallback">Mizu</div>
        {/if}
      </div>

      <div class="detail-main-copy">
        <h1>{show.sonarr_title}</h1>
        <p class="detail-subtitle">{subtitle}</p>

        <div class="detail-hero-chips">
          <span class="detail-chip"><IconWaveSineRegular size={13} weight="regular" /> {markerSummary}</span>
          <span class="detail-chip" class:state-on={show.enabled} class:state-off={!show.enabled}>{show.enabled ? 'Sync on' : 'Sync off'}</span>
          <span class="detail-chip">{show.auto_matched ? 'Auto match' : 'Manual match'}</span>
        </div>

        <div class="detail-divider"></div>

        <div class="rule-pill-row" role="group" aria-label="Tracking rules">
          <label class="rule-pill" class:active={trackAnimeCanon}>
            <input type="checkbox" bind:checked={trackAnimeCanon} />
            <span>Anime canon</span>
          </label>
          <label class="rule-pill" class:active={trackMixed}>
            <input type="checkbox" bind:checked={trackMixed} />
            <span>Mixed canon</span>
          </label>
          <label class="rule-pill" class:active={trackFiller}>
            <input type="checkbox" bind:checked={trackFiller} />
            <span>Filler</span>
          </label>
        </div>
      </div>

      <aside class="detail-marker-panel">
        <span class="marker-label">Progress cutoff</span>
        <div class="marker-segmented">
          <button type="button" class:active={watermarkMode === WATERMARK_AUTO} on:click={() => (watermarkMode = WATERMARK_AUTO)}>Auto</button>
          <button type="button" class:active={watermarkMode === WATERMARK_MANUAL} on:click={() => (watermarkMode = WATERMARK_MANUAL)}>Manual</button>
        </div>

        {#if watermarkMode === WATERMARK_AUTO}
          <div class="marker-display-card">
            <strong>{markerSummary}</strong>
            <span>Follows current Sonarr progress and advances automatically.</span>
          </div>
        {:else}
          <div class="form-group marker-form-group">
            <label class="form-label" for="marker-value">Episode cutoff</label>
            <input id="marker-value" class="form-input detail-number-input" type="number" min="0" bind:value={markerValue} />
            <div class="form-hint">Episodes at or below this number stay unmonitored.</div>
          </div>
        {/if}
      </aside>
    </section>

    <section class="stats-grid detail-stats">
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'all'} on:click={() => (epFilter = 'all')}>
        <span class="stat-label">All episodes</span>
        <span class="stat-value">{epStats.total}</span>
        <span class="stat-sub">Hydrated from Sonarr + AFL</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'canon'} on:click={() => (epFilter = 'canon')}>
        <span class="stat-label" style="color:var(--canon)">Canon</span>
        <span class="stat-value" style="color:var(--canon)">{epStats.canon}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'anime_canon'} on:click={() => (epFilter = 'anime_canon')}>
        <span class="stat-label" style="color:var(--anime-canon)">Anime canon</span>
        <span class="stat-value" style="color:var(--anime-canon)">{epStats.animeCanon}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'mixed_canon'} on:click={() => (epFilter = 'mixed_canon')}>
        <span class="stat-label" style="color:var(--mixed)">Mixed</span>
        <span class="stat-value" style="color:var(--mixed)">{epStats.mixed}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'filler'} on:click={() => (epFilter = 'filler')}>
        <span class="stat-label" style="color:var(--filler)">Filler</span>
        <span class="stat-value" style="color:var(--filler)">{epStats.filler}</span>
      </button>
    </section>

    {#if episodes.length === 0}
      <div class="card detail-empty">
        <div class="empty-state compact-empty">
          <div class="empty-icon"><IconWaveSineRegular size={28} weight="regular" /></div>
          <h3>No episode data yet</h3>
          <p>Episode data has not been hydrated for this anime.</p>
        </div>
      </div>
    {:else}
      {#each groupedBySeason as group}
        <div class="season-group">
          <button class="season-header" on:click={() => toggleSeason(group.season)}>
            <span class="season-chevron" class:collapsed={collapsedSeasons[group.season]}>&#9660;</span>
            <h3 class="season-title">{seasonLabel(group.season)}</h3>
            <span class="season-count">{group.episodes.length} episodes</span>
          </button>
          {#if !collapsedSeasons[group.season]}
            <div class="table-container">
              <table>
                <thead>
                  <tr>
                    <th style="width:76px">#</th>
                    <th>Title</th>
                    <th style="width:128px">Type</th>
                    <th style="width:112px">Monitored</th>
                    <th style="width:88px">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {#each group.episodes as ep}
                    {@const sonarrEp = sonarrByAbsNum[ep.episode_num]}
                    <tr class={epRowClass(ep.type)}>
                      <td style="font-weight:700;font-variant-numeric:tabular-nums">{ep.episode_num}</td>
                      <td>{ep.title || '-'}</td>
                      <td><Badge type={ep.type} /></td>
                      <td>
                        {#if sonarrEp}
                          {#if sonarrEp.monitored}<Badge type="success" label="Yes" />{:else}<Badge type="unknown" label="No" />{/if}
                        {:else}
                          <span class="form-hint">--</span>
                        {/if}
                      </td>
                      <td>
                        {#if sonarrEp}
                          <button
                            class="episode-monitor-btn"
                            class:is-monitored={sonarrEp.monitored}
                            disabled={syncingEps[sonarrEp.id]}
                            title={sonarrEp.monitored ? 'Unmonitor episode' : 'Monitor episode'}
                            aria-label={sonarrEp.monitored ? 'Unmonitor episode' : 'Monitor episode'}
                            on:click={() => toggleEpisodeMonitor(ep)}
                          >
                            {#if syncingEps[sonarrEp.id]}
                              <span class="spinner"></span>
                            {:else if sonarrEp.monitored}
                              <IconEyeRegular size={16} weight="regular" />
                            {:else}
                              <IconEyeClosedRegular size={16} weight="regular" />
                            {/if}
                          </button>
                        {/if}
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
{/if}

<Modal title="Remove Anime" show={showDeleteModal} on:close={() => !deleting && (showDeleteModal = false)}>
  <p>This removes <strong>{show?.sonarr_title}</strong> from Mizu and deletes its stored episode sync data. Sonarr itself is not deleted.</p>
  <svelte:fragment slot="footer">
    <button class="btn btn-secondary" on:click={() => (showDeleteModal = false)} disabled={deleting}>Cancel</button>
    <button class="btn btn-danger" on:click={handleDelete} disabled={deleting}>{deleting ? 'Removing...' : 'Remove Anime'}</button>
  </svelte:fragment>
</Modal>

<style>
  .detail-page-shell {
    display: grid;
    gap: 18px;
    max-width: 1160px;
    margin: 0 auto;
  }

  .detail-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    flex-wrap: wrap;
  }

  .detail-backlink {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .detail-actions-wide {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .detail-actions-cluster {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
  }

  .btn-inline-icon {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .detail-top-card {
    display: grid;
    grid-template-columns: 148px minmax(0, 1fr) 280px;
    gap: 18px;
    padding: 18px;
    align-items: start;
  }

  .detail-summary-media {
    border-radius: 20px;
    overflow: hidden;
    min-height: 212px;
    background: linear-gradient(145deg, #0a1d46 0%, #0d5b94 48%, #b6d7e4 100%);
  }

  .detail-summary-media img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }

  .detail-summary-media.no-art {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .detail-fallback {
    font-family: 'Figtree', sans-serif;
    font-size: 30px;
    font-weight: 700;
    color: #f3fbff;
  }

  .detail-main-copy {
    min-width: 0;
  }

  .detail-main-copy h1 {
    margin-bottom: 6px;
    font-size: clamp(1.65rem, 2vw, 2.15rem);
  }

  .detail-subtitle {
    color: var(--text-secondary);
  }

  .detail-hero-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 14px;
  }

  .detail-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    border-radius: 999px;
    background: rgba(11, 26, 44, 0.72);
    border: 1px solid rgba(118, 167, 203, 0.14);
    color: #e4f1fb;
    font-size: 12px;
    font-weight: 600;
  }

  .detail-chip.state-on {
    background: rgba(34, 197, 94, 0.14);
    border-color: rgba(74, 222, 128, 0.22);
    color: #dcfce7;
  }

  .detail-chip.state-off {
    background: rgba(239, 68, 68, 0.14);
    border-color: rgba(248, 113, 113, 0.22);
    color: #fecaca;
  }

  .detail-divider {
    height: 1px;
    margin: 14px 0 12px;
    background: rgba(118, 167, 203, 0.12);
  }

  .rule-pill-row {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .rule-pill {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 40px;
    padding: 0 14px;
    border-radius: 999px;
    border: 1px solid rgba(118, 167, 203, 0.18);
    background: rgba(255, 255, 255, 0.03);
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .rule-pill input {
    position: absolute;
    inset: 0;
    opacity: 0;
    pointer-events: none;
  }

  .rule-pill.active {
    background: rgba(18, 168, 216, 0.16);
    border-color: rgba(118, 167, 203, 0.28);
    color: #eff9ff;
  }

  .detail-marker-panel {
    display: grid;
    gap: 12px;
    padding: 14px;
    border-radius: 20px;
    border: 1px solid rgba(118, 167, 203, 0.14);
    background: rgba(255, 255, 255, 0.03);
  }

  .marker-label {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .marker-segmented {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .marker-segmented button {
    min-height: 38px;
    border-radius: 12px;
    border: 1px solid rgba(118, 167, 203, 0.16);
    background: rgba(10, 18, 30, 0.76);
    color: var(--text-secondary);
    font-weight: 600;
  }

  .marker-segmented button.active {
    color: #f3fbff;
    background: rgba(18, 168, 216, 0.16);
    border-color: rgba(118, 167, 203, 0.24);
  }

  .marker-display-card {
    display: grid;
    gap: 6px;
    padding: 12px 14px;
    border-radius: 16px;
    background: rgba(18, 168, 216, 0.1);
    border: 1px solid rgba(118, 167, 203, 0.16);
  }

  .marker-display-card strong {
    font-family: 'Figtree', sans-serif;
    font-size: 1rem;
  }

  .marker-display-card span {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .marker-form-group {
    margin-bottom: 0;
  }

  .detail-number-input {
    max-width: 160px;
  }

  .detail-stats {
    margin-bottom: 0;
  }

  .stat-button {
    border: 1px solid var(--border);
    text-align: left;
    cursor: pointer;
  }

  .stat-button.active-filter {
    border-color: rgba(102, 195, 225, 0.34);
    background: rgba(18, 168, 216, 0.1);
  }

  .detail-empty {
    min-height: 280px;
  }

  .compact-empty {
    display: grid;
    gap: 12px;
    place-items: center;
    text-align: center;
    padding: 28px;
  }

  .compact-empty .empty-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 54px;
    height: 54px;
    border-radius: 18px;
    background: rgba(18, 168, 216, 0.14);
    color: var(--accent);
  }

  .season-group {
    margin-bottom: 14px;
  }

  .season-header {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 12px 14px;
    background: linear-gradient(135deg, rgba(10, 17, 28, 0.96), rgba(13, 30, 49, 0.96));
    border: 1px solid rgba(118, 167, 203, 0.16);
    border-radius: 16px 16px 0 0;
    color: var(--text-primary);
    text-align: left;
  }

  .season-chevron {
    color: var(--text-secondary);
    transition: transform var(--transition);
  }

  .season-chevron.collapsed {
    transform: rotate(-90deg);
  }

  .season-title {
    margin: 0;
    font-size: 14px;
  }

  .season-count {
    margin-left: auto;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .episode-monitor-btn {
    width: 34px;
    height: 34px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 999px;
    border: 1px solid rgba(118, 167, 203, 0.18);
    background: rgba(10, 17, 29, 0.82);
    color: var(--text-secondary);
    transition: all var(--transition);
  }

  .episode-monitor-btn:hover:not(:disabled) {
    border-color: rgba(102, 195, 225, 0.34);
    color: #eef7ff;
    background: rgba(18, 168, 216, 0.12);
  }

  .episode-monitor-btn.is-monitored {
    color: #dcfce7;
    background: rgba(34, 197, 94, 0.14);
    border-color: rgba(74, 222, 128, 0.22);
  }

  @media (max-width: 980px) {
    .detail-actions-wide {
      flex-direction: column;
      align-items: stretch;
    }

    .detail-actions-cluster {
      justify-content: flex-start;
    }

    .detail-top-card {
      grid-template-columns: 1fr;
    }

    .detail-summary-media {
      max-width: 220px;
    }
  }

  @media (max-width: 720px) {
    .detail-actions-cluster {
      width: 100%;
      flex-direction: column;
    }
  }
</style>
