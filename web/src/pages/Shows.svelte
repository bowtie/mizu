<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { shows, settings, addNotification } from '../lib/stores.js';
  import { getShows, getStatus, getSonarrSeries, triggerSync, searchAFL, suggestAFL, confirmMatch, getSettings } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import Modal from '../components/Modal.svelte';
  import IconMagnifyingGlassRegular from 'phosphor-icons-svelte/IconMagnifyingGlassRegular.svelte';
  import IconPlusRegular from 'phosphor-icons-svelte/IconPlusRegular.svelte';
  import IconCheckCircleRegular from 'phosphor-icons-svelte/IconCheckCircleRegular.svelte';
  import IconWarningCircleRegular from 'phosphor-icons-svelte/IconWarningCircleRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';
  import IconCheckRegular from 'phosphor-icons-svelte/IconCheckRegular.svelte';
  import IconXRegular from 'phosphor-icons-svelte/IconXRegular.svelte';

  const START_FROM_BEGINNING = 'from_beginning';
  const START_FROM_CURRENT = 'from_current_progress';

  let loading = true;
  let filter = '';
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
    } catch (err) {
      addNotification('Failed to load Mizu anime: ' + err.message, 'error');
    }
    loading = false;
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

  function markerChip(show) {
    return show.sync_watermark > 0 ? `#${show.sync_watermark}` : '#0';
  }

  function syncChip(show) {
    return show.enabled ? 'Syncing' : 'Paused';
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
      addNotification('Failed to sync anime: ' + err.message, 'error');
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
  <title>Mizu - Anime</title>
</svelte:head>

<div class="library-shell">
  <section class="page-heading anime-heading">
    <div>
      <h1>Anime</h1>
      <p>{($shows || []).length} synced in Mizu</p>
    </div>
    <button class="btn btn-primary anime-heading-action" on:click={openAddModal} aria-label="Add anime" title="Add anime">
      <IconPlusRegular size={18} weight="regular" />
      <span>Add Anime</span>
    </button>
  </section>

  <label class="search-shell anime-search-row" aria-label="Search synced anime">
    <IconMagnifyingGlassRegular size={18} weight="regular" />
    <input class="landing-search" type="text" placeholder="Search anime..." bind:value={filter} />
  </label>

  {#if loading}
    <Spinner center size="lg" message="Loading Mizu anime..." />
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
        <p>Everything in Sonarr is already synced, or Sonarr returned no anime series.</p>
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
          <div class="form-hint" style="margin-top:8px">No strong automatic suggestions were found for this anime.</div>
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
          <div class="rule-pill-row" role="group" aria-label="Initial tracking rules">
            <label class="rule-pill rule-pill-anime" class:active={addTrackAnimeCanon}>
              <input type="checkbox" bind:checked={addTrackAnimeCanon} />
              {#if addTrackAnimeCanon}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
              <span>Anime canon</span>
            </label>
            <label class="rule-pill rule-pill-mixed" class:active={addTrackMixed}>
              <input type="checkbox" bind:checked={addTrackMixed} />
              {#if addTrackMixed}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
              <span>Mixed canon</span>
            </label>
            <label class="rule-pill rule-pill-filler" class:active={addTrackFiller}>
              <input type="checkbox" bind:checked={addTrackFiller} />
              {#if addTrackFiller}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
              <span>Filler</span>
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
        {#if confirming}<span class="spinner"></span> Syncing...{:else}Confirm match{/if}
      </button>
    {/if}
  </svelte:fragment>
</Modal>

<style>
  .library-shell {
    display: grid;
    gap: 20px;
    max-width: 960px;
    margin: 0 auto;
  }

  .anime-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
  }

  .anime-heading h1 {
    margin-bottom: 4px;
    font-size: 2rem;
    line-height: 1.1;
  }

  .anime-heading p {
    color: var(--text-secondary);
  }

  .anime-heading-action {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .search-shell {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 44px;
    padding: 0 14px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: rgba(12, 26, 46, 0.92);
    color: var(--text-muted);
  }

  .anime-search-row {
    width: 100%;
  }

  .landing-search {
    width: 100%;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font: inherit;
    color: var(--text-primary);
    caret-color: var(--accent-cream);
  }

  .landing-search::placeholder {
    color: var(--text-muted);
  }

  .shows-grid {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 16px;
  }

  .show-card {
    overflow: hidden;
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: rgba(7, 14, 24, 0.94);
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
    transition: border-color var(--transition), background var(--transition);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
  }

  .show-card:hover {
    border-color: var(--border-light);
    transform: translateY(-2px);
    box-shadow: 0 20px 42px rgba(3, 10, 20, 0.34);
  }

  .show-card-media {
    position: relative;
    min-height: 340px;
    overflow: hidden;
    background: linear-gradient(180deg, #1d3558 0%, #10203a 100%);
  }

  .show-card-media::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(180deg, rgba(3, 8, 16, 0.02) 16%, rgba(3, 8, 16, 0.2) 44%, rgba(3, 8, 16, 0.9) 100%);
  }

  .show-card-media img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .show-card-media.no-art {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .show-card-fallback {
    width: 72px;
    height: 72px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(252, 250, 242, 0.08);
    border: 1px solid rgba(252, 250, 242, 0.14);
    font-family: 'Figtree', sans-serif;
    font-size: 1.35rem;
    font-weight: 700;
    color: var(--accent-cream);
    position: relative;
    z-index: 1;
  }

  .show-card-overlay {
    position: absolute;
    inset: auto 0 0;
    z-index: 1;
    display: grid;
    gap: 8px;
    padding: 14px;
  }

  .show-card-copy h3 {
    font-size: 1.1rem;
    line-height: 1.18;
    color: var(--accent-cream);
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
    min-height: 26px;
    padding: 0 9px;
    border-radius: 999px;
    background: rgba(9, 20, 34, 0.9);
    border: 1px solid rgba(138, 182, 244, 0.14);
    color: var(--accent-cream);
    font-size: 11px;
    font-weight: 700;
  }

  .info-chip-on {
    background: var(--success-bg);
    border-color: rgba(111, 219, 141, 0.3);
    color: #eefbe9;
  }

  .info-chip-off {
    background: var(--danger-bg);
    border-color: rgba(243, 93, 78, 0.3);
    color: #ffe3dd;
  }

  .landing-empty {
    min-height: 240px;
  }

  .compact-empty {
    display: grid;
    gap: 12px;
    place-items: center;
    text-align: center;
    padding: 24px;
  }

  .compact-empty .empty-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border-radius: 10px;
    background: rgba(115, 175, 212, 0.12);
    color: var(--accent-blue);
  }

  .modal-copy {
    margin-bottom: 12px;
    font-size: 13px;
    color: var(--text-secondary);
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
    border-radius: 12px;
  }

  .selected-card {
    border-color: rgba(115, 175, 212, 0.22);
    background: rgba(71, 180, 229, 0.08);
  }

  .selectable-title {
    font-family: 'Figtree', sans-serif;
    font-size: 15px;
  }

  .selectable-title.emphasis {
    font-size: 15px;
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
    border-radius: 12px;
    border: 1px solid var(--border);
    background: rgba(12, 26, 46, 0.8);
  }

  .review-panel h3 {
    margin-bottom: 12px;
    font-size: 15px;
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
    gap: 8px;
    min-height: 38px;
    padding: 0 12px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: rgba(25, 40, 62, 0.74);
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

  .rule-pill-anime.active {
    background: rgba(115, 175, 212, 0.1);
    border-color: rgba(115, 175, 212, 0.22);
    color: var(--text-primary);
  }

  .rule-pill-mixed.active {
    background: rgba(115, 175, 212, 0.08);
    border-color: rgba(115, 175, 212, 0.18);
    color: var(--text-primary);
  }

  .rule-pill-filler.active {
    background: var(--filler-bg);
    border-color: rgba(240, 111, 99, 0.32);
    color: #fff6f4;
  }

  .rule-pill-filler:not(.active) {
    border-color: rgba(240, 111, 99, 0.24);
    color: #f3c0bb;
  }

  .match-search-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
    align-items: center;
  }

  @media (max-width: 1100px) {
    .library-shell {
      gap: 18px;
    }

    .shows-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .show-card-media {
      min-height: 320px;
    }
  }

  @media (max-width: 960px) {
    .shows-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 840px) {
    .anime-heading {
      align-items: flex-start;
    }

    .shows-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .show-card-media {
      min-height: 300px;
    }
  }

  @media (max-width: 560px) {
    .library-shell {
      gap: 16px;
    }

    .anime-heading {
      flex-direction: column;
      align-items: stretch;
    }

    .anime-heading-action {
      width: 100%;
      justify-content: center;
    }

    .search-shell {
      min-height: 42px;
      padding: 0 12px;
    }

    .shows-grid {
      grid-template-columns: 1fr;
    }

    .show-card-media {
      min-height: 280px;
    }

    .match-search-row {
      grid-template-columns: 1fr;
    }

    .rule-pill-row {
      display: grid;
      grid-template-columns: 1fr;
    }
  }
</style>

