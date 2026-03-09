<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { shows, settings, addNotification } from '../lib/stores.js';
  import { getShows, getStatus, getSonarrSeries, triggerSync, searchAFL, suggestAFL, confirmMatch, getSettings } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import Modal from '../components/Modal.svelte';
  import IconMagnifyingGlassRegular from 'phosphor-icons-svelte/IconMagnifyingGlassRegular.svelte';
  import IconArrowsClockwiseRegular from 'phosphor-icons-svelte/IconArrowsClockwiseRegular.svelte';
  import IconPlusRegular from 'phosphor-icons-svelte/IconPlusRegular.svelte';
  import IconCheckCircleRegular from 'phosphor-icons-svelte/IconCheckCircleRegular.svelte';
  import IconWarningCircleRegular from 'phosphor-icons-svelte/IconWarningCircleRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';

  const START_FROM_BEGINNING = 'from_beginning';
  const START_FROM_CURRENT = 'from_current_progress';

  let loading = true;
  let syncing = false;
  let filter = '';
  let lastUpdatedAt = null;
  let sonarrConnected = false;

  let allSeries = [];
  let seriesById = {};

  let showAddModal = false;
  let addStep = 'series';
  let sonarrLoading = false;
  let selectedSonarr = null;
  let suggestedMatches = [];
  let displayedMatches = [];
  let matchLoading = false;
  let manualQuery = '';
  let manualSearching = false;
  let manualSearchVisible = false;
  let selectedAfl = null;
  let confirming = false;
  let addTrackFiller = false;
  let addTrackMixed = true;
  let addTrackAnimeCanon = true;
  let addStartMode = START_FROM_CURRENT;

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  $: mappedShows = ($shows || []).map((show) => {
    const series = seriesById[show.sonarr_series_id] || null;
    return {
      ...show,
      poster: posterUrl(series),
      episodeCount: series?.statistics?.episodeCount ?? null,
      fileCount: series?.statistics?.episodeFileCount ?? null,
      seriesStatus: series?.status ?? '',
    };
  });

  $: filteredShows = mappedShows.filter((show) => {
    if (!filter) return true;
    const query = filter.toLowerCase();
    return (show.sonarr_title || '').toLowerCase().includes(query) || (show.afl_title || show.afl_slug || '').toLowerCase().includes(query);
  });

  $: availableSeries = allSeries
    .filter((series) => !new Set(($shows || []).map((show) => show.sonarr_series_id)).has(series.id))
    .sort((a, b) => a.title.localeCompare(b.title));

  onMount(async () => {
    await loadLanding();
  });

  async function loadLanding() {
    loading = true;
    try {
      const [showsRes, statusRes, seriesRes] = await Promise.allSettled([getShows(), getStatus(), getSonarrSeries()]);
      const nextShows = showsRes.status === 'fulfilled' ? (showsRes.value || []) : [];
      shows.set(nextShows);
      sonarrConnected = statusRes.status === 'fulfilled' && statusRes.value?.status === 'ok';
      allSeries = seriesRes.status === 'fulfilled' ? (seriesRes.value || []) : [];
      seriesById = Object.fromEntries(allSeries.map((series) => [series.id, series]));
      lastUpdatedAt = nextShows.reduce((latest, show) => {
        if (!show.updated_at) return latest;
        const time = new Date(show.updated_at).getTime();
        return time > latest ? time : latest;
      }, 0);
    } catch (err) {
      addNotification('Failed to load Mizu library: ' + err.message, 'error');
    }
    loading = false;
  }

  async function handleSync() {
    syncing = true;
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      setTimeout(loadLanding, 1800);
    } catch (err) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
  }

  function posterUrl(series) {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'poster') || series.images.find((image) => image.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    if (preferred.remoteUrl) return preferred.remoteUrl;
    if (preferred.url && /^https?:/i.test(preferred.url)) return preferred.url;
    return '';
  }

  function initials(title) {
    return (title || 'Mizu').split(/\s+/).slice(0, 2).map((part) => part[0] || '').join('').toUpperCase();
  }

  function fileStat(show) {
    if (show.fileCount == null && show.episodeCount == null) return 'Episodes --';
    if (show.fileCount == null) return `${show.episodeCount} eps`;
    if (show.episodeCount == null) return `${show.fileCount} files`;
    return `${show.fileCount}/${show.episodeCount} files`;
  }

  function markerChip(show) {
    return show.sync_watermark > 0 ? `#${show.sync_watermark}` : '#0';
  }

  function syncChip(show) {
    return show.enabled ? 'Sync on' : 'Sync off';
  }

  function formatUpdated(time) {
    if (!time) return 'Never';
    return new Date(time).toLocaleDateString();
  }

  function openShow(showId) {
    push(`/shows/${showId}`);
  }

  async function refreshSettings() {
    try {
      const current = await getSettings();
      settings.set(current || {});
    } catch (err) {
      addNotification('Failed to load settings: ' + err.message, 'error');
    }
  }

  function resetAddRuleDefaults(current = $settings || {}) {
    addTrackFiller = actionToTracked(current.default_filler_action, false);
    addTrackMixed = actionToTracked(current.default_mixed_canon_action, true);
    addTrackAnimeCanon = actionToTracked(current.default_anime_canon_action, true);
    addStartMode = START_FROM_CURRENT;
  }

  async function openAddModal() {
    addStep = 'series';
    selectedSonarr = null;
    suggestedMatches = [];
    displayedMatches = [];
    selectedAfl = null;
    manualQuery = '';
    manualSearchVisible = false;
    showAddModal = true;

    await refreshSettings();
    resetAddRuleDefaults($settings || {});

    sonarrLoading = true;
    try {
      const mapped = new Set(($shows || []).map((show) => show.sonarr_series_id));
      allSeries = allSeries.length ? allSeries : (await getSonarrSeries()) || [];
      seriesById = Object.fromEntries(allSeries.map((series) => [series.id, series]));
      allSeries = allSeries.filter((series) => !mapped.has(series.id));
    } catch (err) {
      addNotification('Failed to load Sonarr series: ' + err.message, 'error');
    }
    sonarrLoading = false;
  }

  async function goToMatchStep() {
    if (!selectedSonarr) return;
    addStep = 'match';
    manualQuery = selectedSonarr.title || '';
    await loadSuggestedMatches();
  }

  async function loadSuggestedMatches() {
    if (!selectedSonarr) return;
    matchLoading = true;
    selectedAfl = null;
    try {
      suggestedMatches = await suggestAFL(selectedSonarr.id);
      displayedMatches = suggestedMatches;
      manualSearchVisible = suggestedMatches.length === 0;
    } catch (err) {
      suggestedMatches = [];
      displayedMatches = [];
      manualSearchVisible = true;
      addNotification('Failed to load AFL suggestions: ' + err.message, 'error');
    }
    matchLoading = false;
  }

  async function handleManualSearch() {
    if (!selectedSonarr || !manualSearchVisible) return;
    manualSearching = true;
    selectedAfl = null;
    try {
      displayedMatches = await searchAFL(manualQuery.trim(), selectedSonarr.id);
    } catch (err) {
      displayedMatches = [];
      addNotification('AFL search failed: ' + err.message, 'error');
    }
    manualSearching = false;
  }

  async function handleConfirmMatch() {
    if (!selectedSonarr || !selectedAfl) return;
    confirming = true;
    try {
      const created = await confirmMatch({
        sonarr_series_id: selectedSonarr.id,
        sonarr_title: selectedSonarr.title,
        afl_slug: selectedAfl.afl_show?.slug || selectedAfl.slug,
        afl_title: selectedAfl.afl_show?.title || selectedAfl.title,
        filler_action: trackedToAction(addTrackFiller),
        mixed_action: trackedToAction(addTrackMixed),
        anime_canon_action: trackedToAction(addTrackAnimeCanon),
        start_mode: addStartMode,
      });
      addNotification(`Synced "${selectedSonarr.title}"`, 'success');
      showAddModal = false;
      await loadLanding();
      if (created?.id) {
        openShow(created.id);
        return;
      }
      const mapped = ($shows || []).find((show) => show.sonarr_series_id === selectedSonarr.id);
      if (mapped) openShow(mapped.id);
    } catch (err) {
      addNotification('Failed to map anime: ' + err.message, 'error');
    }
    confirming = false;
  }

  function matchTitle(match) {
    return match?.afl_show?.title || match?.title || '';
  }

  function matchSlug(match) {
    return match?.afl_show?.slug || match?.slug || '';
  }
</script>

<svelte:head>
  <title>Mizu - Shows</title>
</svelte:head>

<div class="library-shell">
  <section class="page-heading">
    <div>
      <h1>Synced anime</h1>
      <p>{($shows || []).length} synced anime{lastUpdatedAt ? ` • updated ${formatUpdated(lastUpdatedAt)}` : ''}</p>
    </div>
  </section>

  <section class="library-toolbar card">
    <label class="search-shell" aria-label="Search synced anime">
      <IconMagnifyingGlassRegular size={18} weight="regular" />
      <input class="landing-search" type="text" placeholder="Search or jump to an anime..." bind:value={filter} />
    </label>
    <button class="btn btn-secondary toolbar-button" on:click={handleSync} disabled={syncing}>
      <IconArrowsClockwiseRegular size={18} weight="regular" />
      <span>{syncing ? 'Syncing...' : 'Sync'}</span>
    </button>
    <button class="btn btn-primary toolbar-button" on:click={openAddModal}>
      <IconPlusRegular size={18} weight="regular" />
      <span>Add Anime</span>
    </button>
  </section>

  {#if loading}
    <Spinner center size="lg" message="Loading Mizu library..." />
  {:else if filteredShows.length === 0 && ($shows || []).length === 0}
    <div class="card landing-empty">
      <div class="empty-state compact-empty">
        <div class="empty-icon"><IconWaveSineRegular size={28} weight="regular" /></div>
        <h3>No anime synced yet</h3>
        <p>Add your first Sonarr anime mapping to start using Mizu.</p>
        <button class="btn btn-primary" on:click={openAddModal}>
          <IconPlusRegular size={18} weight="regular" />
          <span>Add Anime</span>
        </button>
      </div>
    </div>
  {:else if filteredShows.length === 0}
    <div class="card landing-empty">
      <div class="empty-state compact-empty">
        <div class="empty-icon"><IconWarningCircleRegular size={28} weight="regular" /></div>
        <h3>No matching anime</h3>
        <p>Try another search term.</p>
      </div>
    </div>
  {:else}
    <section class="shows-grid">
      {#each filteredShows as show}
        <button type="button" class="show-card" on:click={() => openShow(show.id)}>
          <div class="show-card-media" class:no-art={!show.poster}>
            {#if show.poster}
              <img src={show.poster} alt={show.sonarr_title} />
            {:else}
              <div class="show-card-fallback">{initials(show.sonarr_title)}</div>
            {/if}
            <div class="show-card-overlay">
              <div class="show-card-copy">
                <h3>{show.sonarr_title}</h3>
              </div>
              <div class="show-card-chips">
                <span class="info-chip"><IconWaveSineRegular size={13} weight="regular" /> {markerChip(show)}</span>
                <span class="info-chip" class:info-chip-on={show.enabled} class:info-chip-off={!show.enabled}><IconCheckCircleRegular size={13} weight="regular" /> {syncChip(show)}</span>
              </div>
            </div>
          </div>
        </button>
      {/each}
    </section>
  {/if}
</div>

<Modal title="Add Anime" show={showAddModal} on:close={() => (showAddModal = false)} size="large">
  {#if addStep === 'series'}
    <p class="modal-copy">Choose the Sonarr anime you want to sync with Mizu.</p>
    {#if sonarrLoading}
      <Spinner center message="Loading Sonarr library..." />
    {:else if availableSeries.length === 0}
      <div class="empty-state compact-empty" style="padding:20px">
        <h3>No unmapped series</h3>
        <p>Everything in Sonarr is already mapped, or Sonarr returned no anime series.</p>
      </div>
    {:else}
      <div class="select-list">
        {#each availableSeries as series}
          <button type="button" class="card selectable-card" class:selected-card={selectedSonarr?.id === series.id} on:click={() => (selectedSonarr = series)}>
            <div class="selectable-title">{series.title}</div>
            <div class="selectable-meta">{series.year || 'Anime'}</div>
          </button>
        {/each}
      </div>
    {/if}
  {:else}
    <p class="modal-copy">Match <strong>{selectedSonarr?.title}</strong> to AnimeFillerList.</p>

    {#if matchLoading}
      <Spinner center message="Finding strict matches..." />
    {:else}
      {#if manualSearchVisible}
        <div class="form-group" style="margin-bottom:16px">
          <label class="form-label" for="afl-search">Refine manually</label>
          <div class="match-search-row">
            <input id="afl-search" class="form-input" type="text" placeholder="Search AnimeFillerList" bind:value={manualQuery} on:keydown={(event) => event.key === 'Enter' && handleManualSearch()} />
            <button class="btn btn-secondary" on:click={handleManualSearch} disabled={manualSearching}>
              <IconMagnifyingGlassRegular size={16} weight="regular" />
              <span>{manualSearching ? 'Searching...' : 'Search'}</span>
            </button>
          </div>
          <div class="form-hint" style="margin-top:8px">No strong automatic suggestions were found for this series.</div>
        </div>
      {/if}

      {#if displayedMatches.length === 0}
        <div class="empty-state compact-empty" style="padding:20px">
          <h3>No matches</h3>
          <p>No strict AFL matches were found automatically or from the current manual search.</p>
          <p class="form-hint" style="margin-top:12px">If the anime is new, refresh the AFL show list in Settings and try again.</p>
        </div>
      {:else}
        <div class="select-list compact">
          {#each displayedMatches as result}
            <button type="button" class="card selectable-card" class:selected-card={selectedAfl && matchSlug(selectedAfl) === matchSlug(result)} on:click={() => (selectedAfl = result)}>
              <div class="selectable-title emphasis">{matchTitle(result)}</div>
              <div class="selectable-meta">{matchSlug(result)}</div>
              {#if result.confidence != null}
                <div class="match-score-row">
                  <Badge type="info" label={`${Math.round(result.confidence * 100)}%`} />
                  <span class="selectable-meta">{result.method}</span>
                </div>
              {/if}
            </button>
          {/each}
        </div>
      {/if}

      {#if selectedAfl}
        <div class="review-panel">
          <h3>Initial rules</h3>
          <div class="check-list compact-check-list">
            <label class="check-card compact">
              <input type="checkbox" bind:checked={addTrackAnimeCanon} />
              <div>
                <strong>Track anime canon</strong>
                <small>Monitor anime-only canon episodes above the marker.</small>
              </div>
            </label>
            <label class="check-card compact">
              <input type="checkbox" bind:checked={addTrackMixed} />
              <div>
                <strong>Track mixed canon</strong>
                <small>Monitor mixed canon episodes above the marker.</small>
              </div>
            </label>
            <label class="check-card compact">
              <input type="checkbox" bind:checked={addTrackFiller} />
              <div>
                <strong>Track filler</strong>
                <small>Monitor filler episodes above the marker.</small>
              </div>
            </label>
          </div>
          <div class="form-group" style="margin-top:14px;margin-bottom:0">
            <label class="form-label" for="start-mode">Start point</label>
            <select id="start-mode" class="form-select" bind:value={addStartMode}>
              <option value={START_FROM_CURRENT}>Current progress</option>
              <option value={START_FROM_BEGINNING}>Episode 1</option>
            </select>
            <div class="form-hint">Current progress seeds the marker from Sonarr. Episode 1 starts with no cutoff.</div>
          </div>
        </div>
      {/if}
    {/if}
  {/if}

  <svelte:fragment slot="footer">
    <button class="btn btn-secondary" on:click={() => (showAddModal = false)}>Cancel</button>
    {#if addStep === 'series' && selectedSonarr}
      <button class="btn btn-primary" on:click={goToMatchStep}>Next</button>
    {/if}
    {#if addStep === 'match'}
      <button class="btn btn-secondary" on:click={() => (addStep = 'series')}>Change anime</button>
    {/if}
    {#if addStep === 'match' && selectedAfl}
      <button class="btn btn-primary" on:click={handleConfirmMatch} disabled={confirming}>
        {#if confirming}<span class="spinner"></span> Mapping...{:else}Confirm match{/if}
      </button>
    {/if}
  </svelte:fragment>
</Modal>

<style>
  .library-shell {
    display: grid;
    gap: 20px;
    max-width: 1180px;
    margin: 0 auto;
  }

  .page-heading {
    padding-inline: 4px;
  }

  .page-heading h1 {
    margin-bottom: 4px;
    font-size: clamp(1.75rem, 2vw, 2.1rem);
  }

  .page-heading p {
    color: var(--text-secondary);
  }

  .library-toolbar {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 14px;
    padding: 12px;
    align-items: center;
    border-radius: 28px;
  }

  .search-shell {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 54px;
    padding: 0 18px;
    border-radius: 999px;
    border: 1px solid rgba(118, 167, 203, 0.18);
    background: rgba(10, 18, 31, 0.84);
    box-shadow: inset 0 1px 0 rgba(194, 226, 255, 0.04);
    color: var(--text-muted);
  }

  .landing-search {
    width: 100%;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font: inherit;
  }

  .toolbar-button {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-height: 54px;
    padding-inline: 18px;
  }

  .shows-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 18px;
    align-items: start;
  }

  .show-card {
    overflow: hidden;
    min-width: 0;
    min-height: 360px;
    border: 1px solid rgba(118, 167, 203, 0.16);
    border-radius: 26px;
    background: rgba(7, 14, 24, 0.94);
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
    transition: transform var(--transition), border-color var(--transition), box-shadow var(--transition);
    box-shadow: 0 18px 44px rgba(0, 0, 0, 0.24);
  }

  .show-card:hover {
    transform: translateY(-4px);
    border-color: rgba(102, 195, 225, 0.34);
    box-shadow: 0 24px 54px rgba(0, 0, 0, 0.28);
  }

  .show-card-media {
    position: relative;
    min-height: 360px;
    overflow: hidden;
    background: linear-gradient(145deg, #0d2240 0%, #113e72 45%, #1fa1c6 100%);
  }

  .show-card-media::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(180deg, rgba(3, 10, 18, 0.04) 10%, rgba(3, 10, 18, 0.2) 42%, rgba(3, 10, 18, 0.76) 78%, rgba(3, 10, 18, 0.94) 100%);
  }

  .show-card-media img {
    width: 100%;
    height: 100%;
    display: block;
    object-fit: cover;
  }

  .show-card-media.no-art {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .show-card-fallback {
    width: 88px;
    height: 88px;
    border-radius: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(194, 226, 255, 0.16);
    font-family: 'Figtree', sans-serif;
    font-size: 1.7rem;
    font-weight: 700;
    color: #f4fbff;
    position: relative;
    z-index: 1;
  }

  .show-card-overlay {
    position: absolute;
    inset: auto 0 0;
    z-index: 1;
    display: grid;
    gap: 14px;
    padding: 18px;
  }

  .show-card-copy {
    display: grid;
    gap: 8px;
  }

  .show-card-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .info-chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-height: 28px;
    padding: 0 10px;
    border-radius: 999px;
    background: rgba(9, 22, 38, 0.7);
    border: 1px solid rgba(149, 202, 234, 0.16);
    backdrop-filter: blur(10px);
    color: #dfedf9;
    font-size: 11px;
    font-weight: 700;
  }

  .info-chip-on {
    background: rgba(34, 197, 94, 0.16);
    border-color: rgba(74, 222, 128, 0.2);
    color: #dcfce7;
  }

  .info-chip-off {
    background: rgba(239, 68, 68, 0.16);
    border-color: rgba(248, 113, 113, 0.2);
    color: #fecaca;
  }

  .show-card-copy h3 {
    font-size: 1.08rem;
    line-height: 1.22;
    color: #f6fbff;
    text-shadow: 0 10px 24px rgba(0, 0, 0, 0.42);
  }

  .landing-empty {
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

  .modal-copy {
    font-size: 13px;
    color: var(--text-secondary);
    margin-bottom: 12px;
  }

  .select-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 320px;
    overflow-y: auto;
  }

  .select-list.compact {
    max-height: 260px;
  }

  .selectable-card {
    padding: 14px;
    text-align: left;
    cursor: pointer;
  }

  .selected-card {
    border-color: rgba(102, 195, 225, 0.34);
    box-shadow: inset 0 0 0 1px rgba(102, 195, 225, 0.18);
  }

  .selectable-title {
    font-family: 'Figtree', sans-serif;
    font-size: 15px;
  }

  .selectable-title.emphasis {
    font-size: 16px;
  }

  .selectable-meta {
    margin-top: 4px;
    color: var(--text-secondary);
    font-size: 12px;
  }

  .match-score-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 10px;
  }

  .review-panel {
    margin-top: 16px;
    padding: 16px;
    border-radius: 18px;
    border: 1px solid rgba(118, 167, 203, 0.16);
    background: rgba(10, 17, 29, 0.78);
  }

  .review-panel h3 {
    margin-bottom: 12px;
    font-size: 15px;
  }

  .check-list {
    display: grid;
    gap: 10px;
  }

  .check-card {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 12px;
    align-items: start;
    padding: 12px 14px;
    border-radius: 16px;
    border: 1px solid rgba(149, 202, 234, 0.12);
    background: rgba(255, 255, 255, 0.03);
    cursor: pointer;
  }

  .check-card strong {
    display: block;
    margin-bottom: 4px;
    font-family: 'Figtree', sans-serif;
  }

  .check-card small {
    color: var(--text-secondary);
    font-size: 12px;
  }

  .match-search-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
    align-items: center;
  }

  @media (max-width: 1100px) {
    .shows-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 840px) {
    .library-toolbar {
      grid-template-columns: 1fr;
    }

    .shows-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 560px) {
    .shows-grid {
      grid-template-columns: 1fr;
    }

    .show-card,
    .show-card-media {
      min-height: 320px;
    }

    .match-search-row {
      grid-template-columns: 1fr;
    }
  }
</style>








