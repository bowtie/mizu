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
  import IconCheckRegular from 'phosphor-icons-svelte/IconCheckRegular.svelte';
  import IconXRegular from 'phosphor-icons-svelte/IconXRegular.svelte';

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
      const derivedSeason = episodeSeason(ep);
      const season = Number.isInteger(derivedSeason) ? derivedSeason : -1;
      if (!groups[season]) groups[season] = [];
      groups[season].push(ep);
    }
    return Object.entries(groups)
      .map(([seasonKey, groupedEpisodes]) => {
        const parsedSeason = Number.parseInt(seasonKey, 10);
        const season = Number.isInteger(parsedSeason) ? parsedSeason : -1;
        return {
          season,
          episodes: (groupedEpisodes || [])
            .slice()
            .sort((a, b) => displayEpisodeNumber(b) - displayEpisodeNumber(a)),
        };
      })
      .sort((a, b) => {
        if (a.season === -1) return 1;
        if (b.season === -1) return -1;
        return b.season - a.season;
      });
  })();
  $: epStats = {
    total: episodes.length,
    canon: episodes.filter((ep) => ep.type === 'canon').length,
    mixed: episodes.filter((ep) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep) => ep.type === 'filler').length,
    special: episodes.filter((ep) => ep.type === 'special').length,
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
      const [allShows, allSeries, episodesResult] = await Promise.all([
        getShows(),
        getSonarrSeries(),
        getEpisodes(showId),
      ]);

      show = (allShows || []).find((item) => String(item.id) === String(showId));
      if (!show) {
        addNotification('Anime not found', 'error');
        push('/shows');
        return;
      }

      series = (allSeries || []).find((item) => item.id === show.sonarr_series_id) || null;
      episodes = episodesResult || [];
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
      addNotification(show.enabled ? 'Sync paused' : 'Sync resumed', 'success');
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
    const sonarrEp = resolveSonarrEpisode(ep);
    if (!sonarrEp?.id) {
      addNotification('No Sonarr episode found for ' + episodeLabel(ep), 'error');
      return;
    }
    syncingEps[sonarrEp.id] = true;
    syncingEps = syncingEps;
    try {
      const nextState = !sonarrEp.monitored;
      await setEpisodeMonitor(showId, sonarrEp.id, nextState);
      if (ep?.sonarr_episode_id) {
        ep.monitored = nextState;
        episodes = episodes;
      }
      sonarrEp.monitored = nextState;
      sonarrSeasons = sonarrSeasons;
      addNotification(`${episodeLabel(ep)} ${nextState ? 'monitored' : 'unmonitored'}`, 'success');
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

  function isSpecialSeasonNumber(num) {
    return num === -1 || num === 0;
  }

  function seasonLabel(num) {
    if (isSpecialSeasonNumber(num)) return 'Specials';
    return `Season ${num}`;
  }

  function resolveSonarrEpisode(ep) {
    if (ep?.sonarr_episode_id) {
      return {
        id: ep.sonarr_episode_id,
        monitored: !!ep.monitored,
        seasonNumber: ep.season_number,
        episodeNumber: ep.sonarr_episode_number,
        absoluteEpisodeNumber: ep.absolute_episode_number,
      };
    }
    return sonarrByAbsNum[ep?.episode_num] || null;
  }

  function episodeSeason(ep) {
    const rawSeason = Number.isInteger(ep?.season_number) ? ep.season_number : resolveSonarrEpisode(ep)?.seasonNumber;
    if (isSpecialSeasonNumber(rawSeason)) return -1;
    return Number.isInteger(rawSeason) ? rawSeason : -1;
  }

  function episodeMonitored(ep) {
    const sonarrEp = resolveSonarrEpisode(ep);
    return sonarrEp ? !!sonarrEp.monitored : null;
  }

  function episodeActionId(ep) {
    const sonarrEp = resolveSonarrEpisode(ep);
    return sonarrEp ? sonarrEp.id : null;
  }

  function displayEpisodeNumber(ep) {
    if (isSpecialSeasonNumber(ep?.season_number)) return ep?.sonarr_episode_number || 0;
    return ep?.absolute_episode_number ?? ep?.episode_num ?? ep?.sonarr_episode_number ?? 0;
  }

  function episodeLabel(ep) {
    if (isSpecialSeasonNumber(ep?.season_number)) return `Special ${displayEpisodeNumber(ep)}`;
    return `Episode #${displayEpisodeNumber(ep)}`;
  }

  function epRowClass(type) {
    switch (type) {
      case 'canon': return 'ep-canon';
      case 'mixed_canon': return 'ep-mixed';
      case 'anime_canon': return 'ep-anime-canon';
      case 'filler': return 'ep-filler';
      case 'special': return 'ep-special';
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
    <div class="detail-header-row">
      <a class="detail-backlink" href="#/shows"><IconArrowLeftRegular size={16} weight="regular" /> Back to anime</a>

      <div class="detail-action-bar">
      <button class="btn btn-danger btn-inline-icon action-remove" on:click={() => (showDeleteModal = true)} aria-label="Remove" title="Remove">
        <IconTrashRegular size={16} weight="regular" />
        <span>Remove</span>
      </button>
      <div class="detail-actions-cluster">
        <button class="btn btn-secondary btn-inline-icon action-compact" on:click={handleToggleSync} disabled={togglingSync} aria-label={show.enabled ? 'Pause syncing' : 'Resume syncing'} title={show.enabled ? 'Pause syncing' : 'Resume syncing'}>
          {#if show.enabled}
            <IconPauseRegular size={16} weight="regular" />
            <span>{togglingSync ? 'Pausing...' : 'Pause'}</span>
          {:else}
            <IconPlayRegular size={16} weight="regular" />
            <span>{togglingSync ? 'Resuming...' : 'Resume'}</span>
          {/if}
        </button>
        <button class="btn btn-secondary btn-inline-icon action-compact" on:click={handleResyncAll} disabled={resyncingAll} aria-label="Resync anime" title="Resync anime">
          <IconArrowsClockwiseRegular size={16} weight="regular" />
          <span>{resyncingAll ? 'Working...' : 'Resync'}</span>
        </button>
        <button class="btn btn-primary btn-inline-icon action-compact" on:click={handleSave} disabled={saving} aria-label="Save anime settings" title="Save anime settings">
          <IconFloppyDiskRegular size={16} weight="regular" />
          <span>{saving ? 'Saving...' : 'Save'}</span>
        </button>
      </div>
    </div>
    </div>

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
          <span class="detail-chip" class:state-on={show.enabled} class:state-off={!show.enabled}>{show.enabled ? 'Syncing' : 'Paused'}</span>
          <span class="detail-chip">{show.auto_matched ? 'Auto match' : 'Manual match'}</span>
        </div>

        <div class="detail-divider"></div>

        <div class="rule-pill-row" role="group" aria-label="Tracking rules">
          <label class="rule-pill rule-pill-anime" class:active={trackAnimeCanon}>
            <input type="checkbox" bind:checked={trackAnimeCanon} />
            {#if trackAnimeCanon}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
            <span>Anime canon</span>
          </label>
          <label class="rule-pill rule-pill-mixed" class:active={trackMixed}>
            <input type="checkbox" bind:checked={trackMixed} />
            {#if trackMixed}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
            <span>Mixed canon</span>
          </label>
          <label class="rule-pill rule-pill-filler" class:active={trackFiller}>
            <input type="checkbox" bind:checked={trackFiller} />
            {#if trackFiller}<IconCheckRegular size={14} weight="bold" />{:else}<IconXRegular size={14} weight="bold" />{/if}
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
            <span>Follows current Sonarr progress.</span>
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
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'canon'} on:click={() => (epFilter = 'canon')}>
        <span class="stat-label stat-canon">Canon</span>
        <span class="stat-value stat-canon">{epStats.canon}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'anime_canon'} on:click={() => (epFilter = 'anime_canon')}>
        <span class="stat-label stat-anime-canon">Anime canon</span>
        <span class="stat-value stat-anime-canon">{epStats.animeCanon}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'mixed_canon'} on:click={() => (epFilter = 'mixed_canon')}>
        <span class="stat-label stat-mixed">Mixed</span>
        <span class="stat-value stat-mixed">{epStats.mixed}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'filler'} on:click={() => (epFilter = 'filler')}>
        <span class="stat-label stat-filler">Filler</span>
        <span class="stat-value stat-filler">{epStats.filler}</span>
      </button>
      <button type="button" class="stat-card stat-button" class:active-filter={epFilter === 'special'} on:click={() => (epFilter = 'special')}>
        <span class="stat-label stat-special">Specials</span>
        <span class="stat-value stat-special">{epStats.special}</span>
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
          <button class="season-header" class:collapsed={collapsedSeasons[group.season]} on:click={() => toggleSeason(group.season)}>
            <span class="season-chevron" class:collapsed={collapsedSeasons[group.season]}>&#9660;</span>
            <h3 class="season-title">{seasonLabel(group.season)}</h3>
            <span class="season-count">{group.episodes.length} episodes</span>
          </button>
          {#if !collapsedSeasons[group.season]}
            <div class="table-container desktop-episode-table">
              <table>
                <thead>
                  <tr>
                    <th style="width:72px">#</th>
                    <th>Title</th>
                    <th style="width:120px" class="center-col">Type</th>
                    <th style="width:100px" class="center-col">Monitored</th>
                    <th style="width:78px" class="center-col">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {#each group.episodes as ep}
                    <tr class={epRowClass(ep.type)}>
                      <td class="episode-num">{displayEpisodeNumber(ep)}</td>
                      <td>{ep.title || '-'}</td>
                      <td class="center-col type-cell"><Badge type={ep.type} /></td>
                      <td class="center-col">
                        {#if episodeActionId(ep)}
                          {#if episodeMonitored(ep)}<Badge type="success" label="Yes" />{:else}<Badge type="unknown" label="No" />{/if}
                        {:else}
                          <span class="form-hint">--</span>
                        {/if}
                      </td>
                      <td class="center-col">
                        {#if episodeActionId(ep)}
                          <button
                            class="episode-monitor-btn"
                            class:is-monitored={episodeMonitored(ep)}
                            disabled={syncingEps[episodeActionId(ep)]}
                            title={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                            aria-label={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                            on:click={() => toggleEpisodeMonitor(ep)}
                          >
                            {#if syncingEps[episodeActionId(ep)]}
                              <span class="spinner"></span>
                            {:else if episodeMonitored(ep)}
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

            <div class="mobile-episode-list">
              {#each group.episodes as ep}
                <article class="mobile-episode-card {epRowClass(ep.type)}">
                  <div class="mobile-episode-head">
                    <strong>{episodeLabel(ep)}</strong>
                    <Badge type={ep.type} />
                  </div>
                  <h4>{ep.title || '-'}</h4>
                  <div class="mobile-episode-meta">
                    <span class="mobile-monitor-state">
                      {#if episodeActionId(ep)}
                        {#if episodeMonitored(ep)}
                          <Badge type="success" label="Monitored" />
                        {:else}
                          <Badge type="unknown" label="Paused" />
                        {/if}
                      {:else}
                        <span class="form-hint">--</span>
                      {/if}
                    </span>
                    {#if episodeActionId(ep)}
                      <button
                        class="episode-monitor-btn"
                        class:is-monitored={episodeMonitored(ep)}
                        disabled={syncingEps[episodeActionId(ep)]}
                        title={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                        aria-label={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                        on:click={() => toggleEpisodeMonitor(ep)}
                      >
                        {#if syncingEps[episodeActionId(ep)]}
                          <span class="spinner"></span>
                        {:else if episodeMonitored(ep)}
                          <IconEyeRegular size={16} weight="regular" />
                        {:else}
                          <IconEyeClosedRegular size={16} weight="regular" />
                        {/if}
                      </button>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      {/each}

      <div class="mobile-flat-episode-list">
        {#each filteredEpisodes as ep}
          <article class="mobile-episode-card {epRowClass(ep.type)}">
            <div class="mobile-episode-head">
              <strong>{episodeLabel(ep)}</strong>
              <Badge type={ep.type} />
            </div>
            <h4>{ep.title || '-'}</h4>
            <div class="mobile-episode-meta">
              <span class="mobile-monitor-state">
                {#if episodeActionId(ep)}
                  {#if episodeMonitored(ep)}
                    <Badge type="success" label="Monitored" />
                  {:else}
                    <Badge type="unknown" label="Paused" />
                  {/if}
                {:else}
                  <span class="form-hint">--</span>
                {/if}
              </span>
              {#if episodeActionId(ep)}
                <button
                  class="episode-monitor-btn"
                  class:is-monitored={episodeMonitored(ep)}
                  disabled={syncingEps[episodeActionId(ep)]}
                  title={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                  aria-label={episodeMonitored(ep) ? 'Unmonitor episode' : 'Monitor episode'}
                  on:click={() => toggleEpisodeMonitor(ep)}
                >
                  {#if syncingEps[episodeActionId(ep)]}
                    <span class="spinner"></span>
                  {:else if episodeMonitored(ep)}
                    <IconEyeRegular size={16} weight="regular" />
                  {:else}
                    <IconEyeClosedRegular size={16} weight="regular" />
                  {/if}
                </button>
              {/if}
            </div>
          </article>
        {/each}
      </div>
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
    max-width: 960px;
    margin: 0 auto;
  }

  .detail-header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .detail-backlink {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .detail-top-card {
    display: grid;
    grid-template-columns: 132px minmax(0, 1fr) 250px;
    gap: 18px;
    padding: 18px;
    align-items: center;
  }

  .detail-action-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .detail-actions-cluster {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .btn-inline-icon {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .action-compact {
    min-height: 38px;
    padding-inline: 12px;
  }

  .action-remove {
    min-height: 38px;
    padding-inline: 12px;
  }

  .detail-summary-media {
    border-radius: 14px;
    overflow: hidden;
    min-height: 204px;
    background: linear-gradient(180deg, #1d3558 0%, #10203a 100%);
    border: 1px solid var(--border-light);
    box-shadow: inset 0 1px 0 rgba(198, 224, 255, 0.08);
  }

  .detail-summary-media img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .detail-summary-media.no-art {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .detail-fallback {
    font-family: 'Figtree', sans-serif;
    font-size: 24px;
    font-weight: 700;
    color: var(--text-on-accent);
  }

  .detail-main-copy {
    min-width: 0;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .detail-main-copy h1 {
    margin-bottom: 6px;
    font-size: 2.25rem;
    line-height: 1.04;
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
    min-height: 30px;
    padding: 0 11px;
    border-radius: 999px;
    background: rgba(10, 23, 41, 0.92);
    border: 1px solid rgba(138, 182, 244, 0.16);
    color: var(--text-primary);
    font-size: 12px;
    font-weight: 600;
  }

  .detail-chip.state-on {
    background: var(--success-bg);
    border-color: rgba(111, 219, 141, 0.3);
    color: #eefbe9;
  }

  .detail-chip.state-off {
    background: var(--danger-bg);
    border-color: rgba(243, 93, 78, 0.28);
    color: #ffe3dd;
  }

  .detail-divider {
    height: 1px;
    margin: 14px 0 12px;
    background: rgba(159, 187, 227, 0.1);
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
    min-height: 40px;
    padding: 0 14px;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: rgba(18, 33, 57, 0.88);
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
    background: rgba(106, 182, 242, 0.16);
    border-color: rgba(138, 182, 244, 0.26);
    color: var(--text-primary);
  }

  .rule-pill-mixed.active {
    background: rgba(106, 182, 242, 0.1);
    border-color: rgba(138, 182, 244, 0.18);
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

  .detail-marker-panel {
    display: grid;
    gap: 12px;
    padding: 16px;
    border-radius: 14px;
    border: 1px solid var(--border-light);
    background: linear-gradient(180deg, rgba(15, 30, 53, 0.96) 0%, rgba(11, 24, 44, 0.94) 100%);
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
    min-height: 36px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: rgba(9, 19, 35, 0.86);
    color: var(--text-secondary);
    font-weight: 600;
  }

  .marker-segmented button.active {
    color: var(--text-primary);
    background: rgba(106, 182, 242, 0.18);
    border-color: rgba(138, 182, 244, 0.28);
  }

  .marker-display-card {
    display: grid;
    gap: 6px;
    padding: 12px 14px;
    border-radius: 10px;
    background: rgba(106, 182, 242, 0.1);
    border: 1px solid rgba(138, 182, 244, 0.2);
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

  .detail-marker-panel .form-input {
    background: rgba(15, 32, 54, 0.92);
    border-color: rgba(71, 180, 229, 0.2);
  }

  .detail-stats {
    margin-bottom: 0;
  }

  @media (max-width: 1040px) and (min-width: 721px) {
    .detail-stats {
      grid-template-columns: repeat(5, minmax(0, 1fr));
    }

    .detail-stats :global(.stat-card) {
      min-width: 0;
      padding: 14px 16px;
    }

    .detail-stats :global(.stat-value) {
      font-size: 1.55rem;
    }
  }

  .stat-button {
    border: 1px solid var(--border);
    text-align: left;
    cursor: pointer;
    border-radius: 12px;
  }

  .stat-button.active-filter {
    border-color: rgba(115, 175, 212, 0.2);
    background: rgba(115, 175, 212, 0.08);
  }

  .stat-canon {
    color: var(--canon) !important;
  }

  .stat-anime-canon {
    color: var(--anime-canon) !important;
  }

  .stat-mixed {
    color: var(--mixed) !important;
  }

  .stat-filler {
    color: var(--filler) !important;
  }

  .stat-special {
    color: var(--special) !important;
  }

  .detail-empty {
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

  .season-group {
    margin-bottom: 14px;
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
    background: rgba(8, 25, 45, 0.7);
  }

  .season-header {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 12px 14px;
    background: rgba(12, 26, 46, 0.92);
    border: none;
    border-bottom: 1px solid rgba(159, 187, 227, 0.1);
    color: var(--text-primary);
    text-align: left;
  }

  .season-header.collapsed {
    border-bottom: none;
  }

  .season-group .table-container {
    border: none;
    border-radius: 0;
    background: transparent;
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

  .episode-num {
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }

  .center-col {
    text-align: center;
  }

  .type-cell :global(.badge) {
    min-height: 24px;
    border-radius: 8px;
  }

  .desktop-episode-table {
    display: block;
  }

  .mobile-episode-list {
    display: none;
  }

  .mobile-flat-episode-list {
    display: none;
  }

  .mobile-episode-card {
    display: grid;
    gap: 10px;
    padding: 14px;
    border-top: 1px solid rgba(159, 187, 227, 0.08);
    background: rgba(10, 17, 29, 0.28);
  }

  .mobile-episode-head,
  .mobile-episode-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .mobile-episode-card h4 {
    font-size: 14px;
    line-height: 1.35;
  }

  .episode-monitor-btn {
    width: 32px;
    height: 32px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: rgba(10, 22, 40, 0.84);
    color: var(--text-secondary);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .episode-monitor-btn:hover:not(:disabled) {
    border-color: rgba(115, 175, 212, 0.2);
    color: var(--text-primary);
    background: rgba(115, 175, 212, 0.08);
  }

  .episode-monitor-btn.is-monitored {
    color: #eefbe9;
    background: var(--success-bg);
    border-color: rgba(111, 219, 141, 0.24);
  }

  @media (max-width: 1100px) {
    .detail-top-card {
      grid-template-columns: 180px minmax(0, 1fr);
      grid-template-areas:
        'media copy'
        'marker marker';
      align-items: center;
    }

    .detail-summary-media {
      grid-area: media;
    }

    .detail-main-copy {
      grid-area: copy;
      align-self: center;
    }

    .detail-marker-panel {
      grid-area: marker;
    }
  }

  @media (max-width: 760px) {
    .detail-top-card {
      grid-template-columns: 1fr;
      grid-template-areas:
        'media'
        'copy'
        'marker';
    }

    .detail-action-bar {
      flex-wrap: wrap;
      align-items: stretch;
    }

    .action-remove {
      flex: 1 1 100%;
    }

    .detail-actions-cluster {
      width: 100%;
      justify-content: stretch;
    }

    .detail-summary-media {
      max-width: 240px;
      min-height: 240px;
    }
  }

  @media (max-width: 720px) {
    .detail-page-shell {
      gap: 14px;
    }

    .detail-header-row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: 10px;
    }

    .detail-top-card {
      padding: 14px;
      gap: 14px;
    }

    .detail-stats,
    .season-group,
    .mobile-flat-episode-list,
    .detail-empty {
      display: none;
    }

    .detail-action-bar {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 8px;
      margin-bottom: 0;
    }

    .action-remove {
      width: 48px;
      height: 48px;
      min-height: 48px;
      padding-inline: 0;
      flex: 0 0 auto;
    }

    .detail-actions-cluster {
      width: auto;
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 8px;
    }

    .detail-action-bar .btn-inline-icon {
      justify-content: center;
      padding-inline: 0;
      flex: 0 0 auto;
    }

    .detail-action-bar .btn-inline-icon span {
      display: none;
    }

    .detail-action-bar .btn-inline-icon :global(svg) {
      width: 19px;
      height: 19px;
    }

    .action-compact {
      width: 48px;
      height: 48px;
      min-height: 48px;
      padding-inline: 0;
      font-size: 12px;
      flex: 0 0 auto;
    }

    .detail-summary-media {
      width: 100%;
      max-width: none;
      min-height: 120px;
      max-height: 148px;
    }



    .detail-main-copy h1 {
      font-size: 1.4rem;
      line-height: 1.15;
    }

    .detail-subtitle {
      font-size: 12px;
    }

    .detail-hero-chips {
      gap: 8px;
      margin-top: 12px;
    }

    .detail-chip {
      min-height: 28px;
      padding: 0 8px;
      font-size: 11px;
    }

    .detail-divider {
      margin: 12px 0 10px;
    }

    .rule-pill-row {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
    }

    .rule-pill-row .rule-pill:last-child {
      grid-column: 1 / -1;
    }

    .rule-pill {
      min-width: 0;
      min-height: 38px;
      padding: 0 10px;
      font-size: 12px;
      justify-content: flex-start;
    }

    .detail-marker-panel {
      gap: 10px;
      padding: 12px;
    }

    .detail-stats {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }

    .detail-stats :global(.stat-card) {
      padding: 14px;
      min-width: 0;
    }

    .detail-stats :global(.stat-value) {
      font-size: 1.5rem;
    }

    .desktop-episode-table {
      display: none;
    }

    .season-group {
      display: none;
    }

    .mobile-flat-episode-list {
      display: grid;
      gap: 10px;
    }

    .mobile-episode-meta {
      align-items: flex-start;
    }
  }

  @media (max-width: 560px) {
    .detail-page-shell {
      gap: 12px;
    }

    .detail-top-card {
      padding: 12px;
      gap: 12px;
    }

    .detail-summary-media {
      min-height: 112px;
      max-height: 132px;
    }

    .detail-actions-cluster {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .detail-actions-cluster :global(button:last-child) {
      grid-column: 1 / -1;
    }

    .detail-hero-chips {
      gap: 6px;
    }

    .rule-pill-row {
      grid-template-columns: 1fr;
    }

    .mobile-episode-head,
    .mobile-episode-meta {
      flex-direction: column;
      align-items: flex-start;
    }

    .episode-monitor-btn {
      width: 36px;
      height: 36px;
    }
  }
</style>














