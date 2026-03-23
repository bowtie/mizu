<script>
  import { onMount } from 'svelte';
  import { push } from 'svelte-spa-router';
  import { addNotification } from '../lib/stores.js';
  import { getShows, getSonarrSeries, updateShow, deleteShow, getEpisodes, setEpisodeMonitor } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import Modal from '../components/Modal.svelte';
  import IconFloppyDiskRegular from 'phosphor-icons-svelte/IconFloppyDiskRegular.svelte';
  import IconEyeRegular from 'phosphor-icons-svelte/IconEyeRegular.svelte';
  import IconTrashRegular from 'phosphor-icons-svelte/IconTrashRegular.svelte';
  import IconRobotRegular from 'phosphor-icons-svelte/IconRobotRegular.svelte';
  import IconWaveSineRegular from 'phosphor-icons-svelte/IconWaveSineRegular.svelte';

  export let params = {};

  const WATERMARK_AUTO = 'auto';
  const WATERMARK_MANUAL = 'manual';

  let loading = true;
  let saving = false;
  let deleting = false;
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
  let activeTypeFilters = [];
  let collapsedSeasons = {};
  let syncingEps = {};

  const actionToTracked = (action, fallback = true) => (action || (fallback ? 'monitor' : 'unmonitor')) === 'monitor';
  const trackedToAction = (tracked) => (tracked ? 'monitor' : 'unmonitor');

  $: showId = params.id;
  $: pageTitle = show ? `Mizu - ${show.sonarr_title}` : 'Mizu - Anime';
  $: filteredEpisodes = episodes.filter((ep) => (activeTypeFilters.length === 0 ? true : activeTypeFilters.includes(ep.type)));
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
  $: syncCollapsedSeasons(groupedBySeason);
  $: epStats = {
    total: episodes.length,
    canon: episodes.filter((ep) => ep.type === 'canon').length,
    mixed: episodes.filter((ep) => ep.type === 'mixed_canon').length,
    animeCanon: episodes.filter((ep) => ep.type === 'anime_canon').length,
    filler: episodes.filter((ep) => ep.type === 'filler').length,
    special: episodes.filter((ep) => ep.type === 'special').length,
  };
  $: subtitle = series?.overview || '';

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

  function syncCollapsedSeasons(groups) {
    if (!groups?.length) {
      collapsedSeasons = {};
      return;
    }

    const latestOpenSeason = groups.find((group) => group.season !== -1)?.season ?? groups[0].season;
    const nextCollapsed = {};

    for (const group of groups) {
      if (Object.prototype.hasOwnProperty.call(collapsedSeasons, group.season)) {
        nextCollapsed[group.season] = collapsedSeasons[group.season];
      } else {
        nextCollapsed[group.season] = group.season !== latestOpenSeason;
      }
    }

    const changed =
      Object.keys(nextCollapsed).length !== Object.keys(collapsedSeasons).length ||
      Object.entries(nextCollapsed).some(([season, isCollapsed]) => collapsedSeasons[season] !== isCollapsed);

    if (changed) {
      collapsedSeasons = nextCollapsed;
    }
  }

  function posterUrl(series) {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'poster') || series.images.find((image) => image.coverType === 'fanart') || series.images[0];
    if (!preferred) return '';
    if (preferred.remoteUrl) return preferred.remoteUrl;
    if (preferred.url && /^https?:/i.test(preferred.url)) return preferred.url;
    return '';
  }

  function backdropUrl(series) {
    if (!series?.images?.length) return '';
    const preferred = series.images.find((image) => image.coverType === 'fanart') || series.images.find((image) => image.coverType === 'banner') || series.images.find((image) => image.coverType === 'poster') || series.images[0];
    if (!preferred) return '';
    if (preferred.remoteUrl) return preferred.remoteUrl;
    if (preferred.url && /^https?:/i.test(preferred.url)) return preferred.url;
    return '';
  }

  function isTypeFilterActive(type) {
    if (type === 'all') {
      return activeTypeFilters.length === 0;
    }
    return activeTypeFilters.includes(type);
  }

  function toggleTypeFilter(type) {
    if (type === 'all') {
      activeTypeFilters = [];
      return;
    }

    if (activeTypeFilters.includes(type)) {
      activeTypeFilters = activeTypeFilters.filter((value) => value !== type);
      return;
    }

    activeTypeFilters = [...activeTypeFilters, type];
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
  <Spinner center size="lg" message="Pulling anime details from the current..." />
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
    {#if backdropUrl(series)}
      <div class="detail-page-backdrop" aria-hidden="true" style={`background-image: url("${backdropUrl(series)}");`}></div>
    {:else}
      <div class="detail-page-backdrop detail-page-backdrop-fallback" aria-hidden="true"></div>
    {/if}
    <div class="detail-page-overlay" aria-hidden="true"></div>

	    <section class="page-heading detail-page-heading">
	      <div class="detail-hero-block">
	        <div class="detail-page-title-block">
	          <div class="detail-title-row">
	            <h1>{show.sonarr_title}</h1>
	          </div>
	          {#if subtitle}
	            <p class="detail-overview">{subtitle}</p>
	          {/if}

          <div class="rule-pill-row">
            <div class="rule-segmented" role="group" aria-label="Tracking rules">
              <button type="button" class="rule-segment" class:is-active={trackAnimeCanon} class:is-inactive={!trackAnimeCanon} aria-pressed={trackAnimeCanon} on:click={() => (trackAnimeCanon = !trackAnimeCanon)}>
                <span class="rule-segment-label">Anime</span>
                <span class="rule-segment-state">{trackAnimeCanon ? 'Included' : 'Skipped'}</span>
              </button>
              <button type="button" class="rule-segment" class:is-active={trackMixed} class:is-inactive={!trackMixed} aria-pressed={trackMixed} on:click={() => (trackMixed = !trackMixed)}>
                <span class="rule-segment-label">Mixed</span>
                <span class="rule-segment-state">{trackMixed ? 'Included' : 'Skipped'}</span>
              </button>
              <button type="button" class="rule-segment" class:is-active={trackFiller} class:is-inactive={!trackFiller} aria-pressed={trackFiller} on:click={() => (trackFiller = !trackFiller)}>
                <span class="rule-segment-label">Filler</span>
                <span class="rule-segment-state">{trackFiller ? 'Included' : 'Skipped'}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

		      <aside class="detail-header-side detail-utility-panel">
		        <div class="detail-utility-body">
		          <label class="marker-label" for="marker-value">Start tracking after</label>
		          <div class="marker-input-row">
		            <input
		              id="marker-value"
              class="form-input detail-number-input"
              class:is-automated={watermarkMode === WATERMARK_AUTO}
              type="number"
              min="0"
              bind:value={markerValue}
              disabled={watermarkMode === WATERMARK_AUTO}
            />
	            <button
	              type="button"
	              class="marker-auto-toggle"
	              class:active={watermarkMode === WATERMARK_AUTO}
	              aria-pressed={watermarkMode === WATERMARK_AUTO}
              aria-label={watermarkMode === WATERMARK_AUTO ? 'Disable automatic tracking start' : 'Enable automatic tracking start'}
	              title={watermarkMode === WATERMARK_AUTO ? 'Automatic tracking start enabled' : 'Use Sonarr progress automatically'}
	              on:click={() => (watermarkMode = watermarkMode === WATERMARK_AUTO ? WATERMARK_MANUAL : WATERMARK_AUTO)}
	            >
		              <IconRobotRegular size={20} weight="regular" />
		              <span>Auto</span>
		            </button>
		          </div>
		          <div class="form-hint marker-hint">
		            {#if watermarkMode === WATERMARK_AUTO}
	              Sonarr progress sets the start point automatically.
	            {:else if markerValue > 0}
		              Episodes at or below #{markerValue} stay unmonitored.
	            {:else}
		              Episodes at or below this number stay unmonitored.
		            {/if}
		          </div>
		        </div>

	        <div class="detail-header-actions detail-header-actions-inline">
	          <button class="btn btn-inline-icon action-compact action-remove" on:click={() => (showDeleteModal = true)}>
	            <IconTrashRegular size={20} weight="regular" />
	            <span>Remove</span>
	          </button>
	          <button class="btn btn-primary btn-inline-icon action-compact action-save" on:click={handleSave} disabled={saving}>
	            <IconFloppyDiskRegular size={20} weight="regular" />
	            <span>{saving ? 'Saving...' : 'Save'}</span>
	          </button>
	        </div>
	      </aside>
	    </section>

	    <section class="stats-grid detail-stats">
	      <button type="button" class="filter-card filter-card-all filter-card-summary" class:has-active={activeTypeFilters.length > 0} on:click={() => toggleTypeFilter('all')}>
	        <span class="stat-label">All episodes</span>
	        <span class="stat-value">{epStats.total}</span>
	      </button>
      <button type="button" class="filter-card filter-card-canon" class:active-filter={isTypeFilterActive('canon')} class:dimmed={activeTypeFilters.length > 0 && !isTypeFilterActive('canon')} on:click={() => toggleTypeFilter('canon')}>
        <span class="stat-label">Manga</span>
        <span class="stat-value">{epStats.canon}</span>
      </button>
      <button type="button" class="filter-card filter-card-anime-canon" class:active-filter={isTypeFilterActive('anime_canon')} class:dimmed={activeTypeFilters.length > 0 && !isTypeFilterActive('anime_canon')} on:click={() => toggleTypeFilter('anime_canon')}>
        <span class="stat-label">Anime</span>
        <span class="stat-value">{epStats.animeCanon}</span>
      </button>
      <button type="button" class="filter-card filter-card-mixed" class:active-filter={isTypeFilterActive('mixed_canon')} class:dimmed={activeTypeFilters.length > 0 && !isTypeFilterActive('mixed_canon')} on:click={() => toggleTypeFilter('mixed_canon')}>
        <span class="stat-label">Mixed</span>
        <span class="stat-value">{epStats.mixed}</span>
      </button>
      <button type="button" class="filter-card filter-card-filler" class:active-filter={isTypeFilterActive('filler')} class:dimmed={activeTypeFilters.length > 0 && !isTypeFilterActive('filler')} on:click={() => toggleTypeFilter('filler')}>
        <span class="stat-label">Filler</span>
        <span class="stat-value">{epStats.filler}</span>
      </button>
    </section>

    {#if episodes.length === 0}
      <div class="card detail-empty">
        <div class="empty-state compact-empty">
          <div class="empty-icon"><IconWaveSineRegular size={28} weight="regular" /></div>
          <h3>No episodes yet</h3>
          <p>We have not synced episode details for this anime yet.</p>
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
                          {#if episodeMonitored(ep)}<Badge type="success" label="Yes" />{:else}<Badge type="info" label="No" />{/if}
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
                            {:else}
                              <IconEyeRegular size={20} weight="regular" />
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
                          <Badge type="info" label="Unmonitored" />
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
                        {:else}
                          <IconEyeRegular size={20} weight="regular" />
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
    {/if}
  </div>
{/if}

<Modal title="Remove Anime" show={showDeleteModal} on:close={() => !deleting && (showDeleteModal = false)}>
  <p>This removes <strong>{show?.sonarr_title}</strong> from Mizu and clears its saved episode data. The anime stays in Sonarr.</p>
  <svelte:fragment slot="footer">
    <button class="btn btn-secondary" on:click={() => (showDeleteModal = false)} disabled={deleting}>Cancel</button>
    <button class="btn btn-danger" on:click={handleDelete} disabled={deleting}>{deleting ? 'Removing...' : 'Remove'}</button>
  </svelte:fragment>
</Modal>

<style>
  .detail-page-shell {
    position: relative;
    isolation: isolate;
    overflow: visible;
    display: grid;
    gap: 14px;
    max-width: 960px;
    margin: 0 auto;
  }

  .detail-page-backdrop,
  .detail-page-overlay {
    position: fixed;
    inset: 0;
    pointer-events: none;
  }

  .detail-page-backdrop {
    z-index: -2;
    opacity: 0.62;
    background-position: center 22%;
    background-repeat: no-repeat;
    background-size: cover;
    filter: saturate(1);
  }

  .detail-page-backdrop-fallback {
    background:
      radial-gradient(circle at 18% 22%, color-mix(in srgb, var(--accent-blue) 18%, transparent) 0%, transparent 34%),
      linear-gradient(135deg, color-mix(in srgb, var(--surface-elevated) 84%, var(--selected-bg) 16%) 0%, var(--panel-bg-strong) 72%);
    opacity: 0.8;
  }

  .detail-page-overlay {
    z-index: -1;
    background:
      radial-gradient(circle at 28% 14%, var(--page-aurora-a) 0%, transparent 26%),
      linear-gradient(180deg, rgba(4, 10, 18, 0.58) 0%, var(--page-depth-mid) 18%, rgba(4, 10, 18, 0.84) 42%, rgba(4, 10, 18, 0.94) 72%, rgba(4, 10, 18, 0.985) 100%);
  }

  .detail-page-heading {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 332px;
    align-items: center;
    gap: 22px;
  }

  .detail-hero-block {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: 16px;
    min-width: 0;
    height: 100%;
  }

  .detail-page-title-block {
    display: grid;
    gap: 10px;
    min-width: 0;
    align-content: start;
    padding-left: 14px;
    border-left: 1px solid var(--divider-border-soft);
  }

  .detail-title-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .detail-page-title-block h1 {
    flex: 1 1 auto;
    font-size: 2.15rem;
    line-height: 1.04;
    margin: 0;
  }

  .detail-overview {
    margin: 0;
    color: var(--text-secondary);
    max-width: 62ch;
    line-height: 1.46;
    text-wrap: pretty;
  }

  .detail-header-side {
    position: relative;
    z-index: 1;
    display: grid;
    gap: 12px;
    align-content: start;
  }

  .detail-utility-panel {
    gap: 12px;
    padding: 14px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--panel-bg-strong);
  }

  .detail-utility-body {
    display: grid;
    gap: 8px;
  }

  .detail-header-actions {
    display: flex;
    flex-wrap: nowrap;
    justify-content: flex-end;
    gap: 8px;
  }

  .detail-header-actions-inline {
    padding-top: 12px;
    border-top: 1px solid var(--divider-border-soft);
    justify-content: stretch;
  }

  .action-remove {
    background: var(--danger-bg);
    border-color: var(--danger-border-strong);
    color: var(--danger-foreground);
  }

  .action-remove:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 20%, var(--danger-bg));
    border-color: var(--danger-border-strong);
    color: #fff;
  }

  .btn-inline-icon {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .action-compact {
    min-height: 36px;
    padding-inline: 12px;
  }

  .action-save {
    flex: 1 1 auto;
    justify-content: center;
  }

  .detail-header-actions-inline .action-remove {
    flex: 0 0 auto;
  }

  .rule-pill-row {
    display: grid;
    margin-top: 0;
    padding-top: 14px;
    border-top: 1px solid var(--divider-border-soft);
  }

  .rule-segmented {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0;
    border: 1px solid color-mix(in srgb, var(--text-primary) 12%, transparent);
    border-radius: 10px;
    overflow: hidden;
    background: color-mix(in srgb, var(--surface-elevated) 78%, transparent);
    backdrop-filter: blur(14px) saturate(120%);
    -webkit-backdrop-filter: blur(14px) saturate(120%);
  }

  .rule-segment {
    display: grid;
    gap: 2px;
    justify-items: start;
    padding: 10px 12px;
    border: 0;
    border-right: 1px solid color-mix(in srgb, var(--text-primary) 10%, transparent);
    background: transparent;
    color: var(--text-muted);
    text-align: left;
    cursor: pointer;
    transition: background var(--transition), color var(--transition), border-color var(--transition);
  }

  .rule-segment:last-child { border-right: 0; }

  .rule-segment:focus-visible {
    outline: none;
    position: relative;
    z-index: 1;
    box-shadow: inset 0 0 0 2px var(--focus-ring);
  }

  /* Enabled = green bg */
  .rule-segment.is-active {
    background: var(--success-bg);
    color: var(--success);
  }

  /* Disabled = red bg */
  .rule-segment.is-inactive {
    background: var(--danger-bg);
    color: var(--danger);
  }

  .rule-segment-label {
    font-size: 13px;
    font-weight: 700;
    color: inherit;
  }

  .rule-segment-state {
    font-size: 11px;
    color: inherit;
    opacity: 0.8;
  }

  .marker-label {
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .marker-input-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
  }

  .marker-auto-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-width: 72px;
    height: 40px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: color-mix(in srgb, var(--surface-elevated) 82%, rgba(7, 16, 29, 0.96) 18%);
    color: var(--text-secondary);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .marker-auto-toggle.active {
    background: color-mix(in srgb, var(--selected-bg) 72%, rgba(9, 21, 39, 0.94) 28%);
    border-color: var(--selected-border);
    color: var(--text-primary);
  }

  .detail-number-input {
    max-width: none;
    background: color-mix(in srgb, var(--surface-elevated) 72%, rgba(7, 16, 29, 0.97) 28%);
    border-color: color-mix(in srgb, var(--border) 82%, rgba(255, 255, 255, 0.05) 18%);
    color: var(--text-primary);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.02);
  }

  .detail-number-input.is-automated {
    color: color-mix(in srgb, var(--text-muted) 82%, rgba(255, 255, 255, 0.1) 18%);
    background: color-mix(in srgb, var(--surface-elevated) 54%, rgba(4, 10, 18, 0.99) 46%);
    border-color: color-mix(in srgb, var(--border) 66%, rgba(255, 255, 255, 0.04) 34%);
    box-shadow: none;
    opacity: 0.72;
    cursor: not-allowed;
  }

  .marker-hint {
    margin-top: 0;
    min-height: 18px;
    line-height: 1.35;
  }

  .marker-auto-toggle:hover {
    border-color: var(--interactive-active-border);
  }

  .marker-auto-toggle.active:hover {
    border-color: var(--selected-border-strong, var(--selected-border));
  }

  .marker-form-group {
    margin-bottom: 0;
  }

  .detail-stats {
    grid-template-columns: minmax(196px, 1.34fr) repeat(4, minmax(0, 1fr));
    gap: 8px;
    margin-bottom: 0;
  }

  .filter-card {
    display: grid;
    gap: 4px;
    justify-items: start;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: color-mix(in srgb, var(--surface) 72%, transparent);
    backdrop-filter: blur(18px) saturate(145%);
    -webkit-backdrop-filter: blur(18px) saturate(145%);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
    text-align: left;
    cursor: pointer;
    transition: background var(--transition), border-color var(--transition), opacity var(--transition), filter var(--transition);
  }

  .filter-card .stat-label {
    font-family: 'Overpass', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    margin: 0;
    white-space: nowrap;
    color: var(--text-muted);
    transition: color var(--transition);
  }

  .filter-card .stat-value {
    font-family: 'Figtree', sans-serif;
    font-size: 1.72rem;
    line-height: 1;
    margin: 0;
    font-variant-numeric: tabular-nums;
    font-weight: 800;
    transition: color var(--transition);
  }

  /* ── Default: all type cards vivid (everything is showing) ── */
  .filter-card-canon      { background: color-mix(in srgb, var(--canon) 13%, rgba(5, 13, 26, 0.78)); border-color: color-mix(in srgb, var(--canon-border) 82%, rgba(255, 255, 255, 0.08)); }
  .filter-card-canon      .stat-label { color: var(--canon); }
  .filter-card-canon      .stat-value { color: var(--canon); }

  .filter-card-anime-canon { background: color-mix(in srgb, var(--anime-canon) 13%, rgba(5, 13, 26, 0.78)); border-color: color-mix(in srgb, var(--anime-canon-border) 82%, rgba(255, 255, 255, 0.08)); }
  .filter-card-anime-canon .stat-label { color: var(--anime-canon); }
  .filter-card-anime-canon .stat-value { color: var(--anime-canon); }

  .filter-card-mixed      { background: color-mix(in srgb, var(--mixed) 13%, rgba(5, 13, 26, 0.78)); border-color: color-mix(in srgb, var(--mixed-border) 82%, rgba(255, 255, 255, 0.08)); }
  .filter-card-mixed      .stat-label { color: var(--mixed); }
  .filter-card-mixed      .stat-value { color: var(--mixed); }

  .filter-card-filler     { background: color-mix(in srgb, var(--filler) 14%, rgba(5, 13, 26, 0.78)); border-color: color-mix(in srgb, var(--filler-border-strong) 82%, rgba(255, 255, 255, 0.08)); }
  .filter-card-filler     .stat-label { color: var(--filler); }
  .filter-card-filler     .stat-value { color: var(--filler); }

  /* ── "All episodes" — neutral reset button, always opaque ── */
  .filter-card-all {
    background: color-mix(in srgb, var(--text-primary) 7%, rgba(5, 13, 26, 0.76));
    border-color: color-mix(in srgb, var(--text-primary) 10%, transparent);
  }
  .filter-card-all .stat-value { color: var(--text-primary); }
  .filter-card-all .stat-label { color: var(--text-muted); }

  /* When a filter IS active: All gets a blue hint as reset cue */
  .filter-card-all.has-active {
    background: color-mix(in srgb, var(--blue-base) 14%, var(--bg-dark));
    border-color: var(--accent-border);
  }
  .filter-card-all.has-active .stat-label { color: var(--blue-vivid); }

  /* ── Dimmed: this type is NOT the active filter ── */
  .filter-card.dimmed {
    background: var(--surface);
    border-color: var(--border);
    opacity: 0.4;
    filter: grayscale(0.5);
  }
  .filter-card.dimmed .stat-label,
  .filter-card.dimmed .stat-value { color: var(--text-muted); }

  /* ── Active filter: ring in the type's own color ── */
  .filter-card.active-filter { box-shadow: none; }

  .filter-card-all.active-filter,
  .filter-card-all.has-active {
    box-shadow: 0 0 0 2px var(--accent-border-strong);
  }
  .filter-card-canon.active-filter      { box-shadow: 0 0 0 2px var(--canon-border); border-color: var(--canon-border); }
  .filter-card-anime-canon.active-filter { box-shadow: 0 0 0 2px var(--anime-canon-border); border-color: var(--anime-canon-border); }
  .filter-card-mixed.active-filter      { box-shadow: 0 0 0 2px var(--mixed-border); border-color: var(--mixed-border); }
  .filter-card-filler.active-filter     { box-shadow: 0 0 0 2px var(--filler-border-strong); border-color: var(--filler-border-strong); }

  .filter-card:hover:not(.dimmed) {
    filter: brightness(1.1);
  }
  .filter-card.dimmed:hover {
    opacity: 0.65;
    filter: grayscale(0.2);
  }

  .filter-card-summary {
    position: relative;
    min-height: 100%;
    padding: 14px 20px 14px 14px;
  }

  .filter-card-summary::after {
    content: '';
    position: absolute;
    top: 12px;
    right: 0;
    bottom: 12px;
    width: 1px;
    background: var(--divider-border-soft);
  }

  .filter-card-summary .stat-label { font-size: 10px; letter-spacing: 0.08em; }
  .filter-card-summary .stat-value { font-size: 1.92rem; }

  .detail-empty {
    min-height: 240px;
  }

  .compact-empty {
    display: grid;
    gap: 6px;
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
    background: var(--empty-icon-gradient);
    color: var(--accent-blue);
    box-shadow: 0 12px 26px rgba(125, 200, 240, 0.18);
  }

  .season-group {
    margin-bottom: 10px;
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    background: var(--surface);
  }

  .season-header {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 13px 14px;
    background: var(--surface-elevated);
    border: none;
    border-bottom: 1px solid var(--divider-border);
    color: var(--text-primary);
    text-align: left;
  }

  .season-header.collapsed {
    border-bottom: none;
  }

  .season-group .table-container {
    border: none;
    border-radius: 0;
    background: var(--surface);
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
    font-size: 15px;
    font-weight: 700;
  }

  .season-count {
    margin-left: auto;
    font-size: 13px;
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

  .desktop-episode-table :global(tbody tr) {
    transition: background var(--transition);
  }

  .desktop-episode-table :global(tbody tr:hover) {
    background: var(--table-row-hover);
  }

  .mobile-episode-list {
    display: none;
  }

  .mobile-episode-card {
    display: grid;
    gap: 6px;
    padding: 12px;
    border-top: 1px solid var(--divider-border-soft);
    background: var(--surface-elevated);
  }

  .mobile-episode-head,
  .mobile-episode-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
  }

  .mobile-episode-card h4 {
    font-size: 15px;
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
    background: var(--surface-elevated);
    color: var(--text-secondary);
    transition: background var(--transition), border-color var(--transition), color var(--transition);
  }

  .episode-monitor-btn:hover:not(:disabled) {
    border-color: var(--selected-border);
    color: var(--text-primary);
    background: var(--surface-soft);
  }

  .episode-monitor-btn.is-monitored {
    color: var(--success-foreground);
    background: var(--success-bg);
    border-color: var(--success-border);
  }

  .episode-monitor-btn:not(.is-monitored) {
    background: color-mix(in srgb, var(--info-bg) 64%, rgba(8, 18, 33, 0.78) 36%);
    border-color: var(--info-border);
    color: var(--info);
  }

  @media (max-width: 940px) {
    .detail-page-heading {
      grid-template-columns: minmax(0, 1fr) 272px;
      gap: 14px;
    }

    .detail-header-actions {
      gap: 6px;
    }

    .action-compact {
      min-height: 34px;
      padding-inline: 9px;
      font-size: 13px;
    }

    .btn-inline-icon {
      gap: 5px;
    }

    .btn-inline-icon :global(svg) {
      width: 20px;
      height: 20px;
    }
  }

  @media (max-width: 760px) {
    .detail-page-heading {
      grid-template-columns: 1fr;
      align-items: start;
    }

    .detail-hero-block {
      height: auto;
    }

    .detail-utility-panel {
      width: min(100%, 420px);
      justify-self: start;
    }

    .detail-page-title-block {
      padding-left: 12px;
    }

    .detail-stats {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .filter-card-summary {
      grid-column: 1 / -1;
      padding-right: 12px;
    }

    .filter-card-summary::after {
      display: none;
    }
  }

  @media (max-width: 720px) {
    .detail-page-shell {
      gap: 12px;
    }

    .detail-page-heading {
      gap: 16px;
    }

    .detail-page-title-block {
      padding-left: 10px;
    }

    .detail-header-actions {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
      width: 100%;
    }

    .detail-header-actions .btn-inline-icon {
      flex-direction: column;
      justify-content: center;
      gap: 4px;
      padding: 10px 12px;
      min-height: 56px;
      text-align: center;
    }

    .detail-header-actions .btn-inline-icon span {
      display: inline;
      font-size: 12px;
      line-height: 1;
    }

    .detail-header-actions .btn-inline-icon :global(svg) {
      width: 20px;
      height: 20px;
    }

    .rule-segment {
      padding: 9px 8px;
    }

    .rule-segment-label {
      font-size: 13px;
    }

    .rule-segment-state {
      font-size: 11px;
    }

    .detail-stats {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 6px;
    }

    .filter-card-summary {
      grid-column: 1 / -1;
    }

    .desktop-episode-table {
      display: none;
    }

    .mobile-episode-list {
      display: block;
    }

    .mobile-episode-meta {
      align-items: flex-start;
    }
  }

  @media (max-width: 560px) {
    .detail-page-shell {
      gap: 8px;
    }

    .detail-header-actions {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .detail-utility-panel {
      width: 100%;
    }

    .detail-page-title-block h1 {
      font-size: 1.7rem;
    }

    .detail-page-title-block {
      padding-left: 0;
      border-left: none;
    }

    .detail-title-row {
      gap: 8px;
    }

    .detail-overview {
      max-width: none;
    }

    .filter-card .stat-label {
      white-space: normal;
    }

    .filter-card-summary .stat-value {
      font-size: 1.58rem;
    }

    .rule-segment {
      padding: 8px 6px;
    }

    .rule-segment-label {
      font-size: 12px;
    }

    .rule-segment-state {
      font-size: 10px;
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






































