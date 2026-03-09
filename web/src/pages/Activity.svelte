<script>
  import { onMount } from 'svelte';
  import { addNotification } from '../lib/stores.js';
  import { getStatus, getSyncLogs, getShows, triggerSync } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import IconArrowsClockwiseRegular from 'phosphor-icons-svelte/IconArrowsClockwiseRegular.svelte';
  import IconClockCounterClockwiseRegular from 'phosphor-icons-svelte/IconClockCounterClockwiseRegular.svelte';
  import IconCaretRightRegular from 'phosphor-icons-svelte/IconCaretRightRegular.svelte';
  import IconWarningCircleRegular from 'phosphor-icons-svelte/IconWarningCircleRegular.svelte';

  const PAGE_SIZE = 12;

  let loading = true;
  let logs = [];
  let syncedAnimeCount = 0;
  let sonarrConnected = false;
  let syncing = false;
  let loadingMore = false;
  let hasMore = false;
  let nextOffset = 0;
  let expandedId = null;

  onMount(async () => {
    await loadActivity(true);
  });

  async function loadActivity(reset = true) {
    if (reset) {
      loading = logs.length === 0;
    } else {
      loadingMore = true;
    }

    try {
      const [statusRes, logsRes, showsRes] = await Promise.allSettled([
        getStatus(),
        getSyncLogs(PAGE_SIZE, reset ? 0 : nextOffset),
        getShows(),
      ]);

      sonarrConnected = statusRes.status === 'fulfilled' && statusRes.value?.status === 'ok';
      syncedAnimeCount = showsRes.status === 'fulfilled' ? (showsRes.value || []).length : syncedAnimeCount;

      if (logsRes.status === 'fulfilled') {
        const page = logsRes.value || { items: [], has_more: false, next_offset: 0 };
        logs = reset ? (page.items || []) : [...logs, ...(page.items || [])];
        hasMore = !!page.has_more;
        nextOffset = page.next_offset || logs.length;
      }
    } catch (err) {
      addNotification('Failed to load activity: ' + err.message, 'error');
    }

    loading = false;
    loadingMore = false;
  }

  async function handleSync() {
    syncing = true;
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      setTimeout(() => loadActivity(true), 1800);
    } catch (err) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id;
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return 'Running';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    return Math.floor(ms / 60000) + 'm ' + Math.round((ms % 60000) / 1000) + 's';
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }

  function formatTrigger(trigger) {
    if (!trigger) return '-';
    return trigger === 'manual' ? 'Manual' : 'Scheduled';
  }

  function runStatusType(status) {
    switch (status) {
      case 'success': return 'success';
      case 'partial': return 'partial';
      case 'failed': return 'failed';
      case 'running': return 'running';
      default: return 'unknown';
    }
  }

  function itemStatusType(status) {
    switch (status) {
      case 'success': return 'success';
      case 'failed': return 'failed';
      default: return 'info';
    }
  }

  function latestRunSummary() {
    if (!logs.length) return 'No runs recorded yet';
    const latest = logs[0];
    return `${formatTrigger(latest.trigger)} · ${formatDuration(latest.started_at, latest.completed_at)}`;
  }
</script>

<svelte:head>
  <title>Mizu - Activity</title>
</svelte:head>

<div class="activity-shell">
  <section class="page-heading row-heading compact-heading">
    <div>
      <h1>Activity</h1>
      <p>{logs.length} recent runs • {syncedAnimeCount} synced anime</p>
    </div>
    <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
      <IconArrowsClockwiseRegular size={18} weight="regular" />
      <span>{syncing ? 'Syncing...' : 'Sync now'}</span>
    </button>
  </section>

  {#if loading}
    <Spinner center size="lg" message="Loading Mizu activity..." />
  {:else}
    <section class="activity-strip compact-strip">
      <div class="mini-stat card">
        <span class="mini-label">Latest run</span>
        <strong>{latestRunSummary()}</strong>
        <small>{logs[0] ? formatTime(logs[0].started_at) : 'Trigger a sync to populate activity.'}</small>
      </div>
      <div class="mini-stat card">
        <span class="mini-label">Sonarr</span>
        <strong class:state-online={sonarrConnected} class:state-offline={!sonarrConnected}>{sonarrConnected ? 'Online' : 'Offline'}</strong>
        <small>Current connection status</small>
      </div>
      <div class="mini-stat card">
        <span class="mini-label">Synced anime</span>
        <strong>{syncedAnimeCount}</strong>
        <small>Currently managed by Mizu</small>
      </div>
    </section>

    {#if logs.length === 0}
      <div class="card activity-empty">
        <div class="empty-state compact-empty">
          <div class="empty-icon"><IconClockCounterClockwiseRegular size={28} weight="regular" /></div>
          <h3>No sync activity yet</h3>
          <p>Run a sync to start building a readable history of what Mizu changed.</p>
          <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
            <IconArrowsClockwiseRegular size={18} weight="regular" />
            <span>Sync now</span>
          </button>
        </div>
      </div>
    {:else}
      <section class="card activity-panel concise-panel">
        <div class="panel-head aligned-panel-head">
          <div>
            <h3>Recent runs</h3>
            <p>Per-run summaries with anime-level outcomes.</p>
          </div>
        </div>

        <div class="run-list">
          {#each logs as log}
            <article class="run-item" class:expanded={expandedId === log.id}>
              <button class="run-summary" on:click={() => toggleExpand(log.id)}>
                <span class="run-expand" aria-hidden="true">
                  <IconCaretRightRegular size={16} weight="bold" />
                </span>
                <span class="run-col run-started">
                  <span class="run-label">Started</span>
                  <strong>{formatTime(log.started_at)}</strong>
                </span>
                <span class="run-col">
                  <span class="run-label">Duration</span>
                  <strong>{formatDuration(log.started_at, log.completed_at)}</strong>
                </span>
                <span class="run-col">
                  <span class="run-label">Anime</span>
                  <strong>{log.shows_processed}</strong>
                </span>
                <span class="run-col">
                  <span class="run-label">Episodes</span>
                  <strong>{log.episodes_updated}</strong>
                </span>
                <span class="run-col">
                  <span class="run-label">Trigger</span>
                  <strong>{formatTrigger(log.trigger)}</strong>
                </span>
                <span class="run-status-col">
                  <Badge type={runStatusType(log.status)} label={log.status} />
                </span>
              </button>

              {#if expandedId === log.id}
                <div class="run-detail">
                  {#if log.items?.length}
                    <div class="run-detail-list">
                      {#each log.items as item}
                        <div class="run-detail-item">
                          <div class="run-detail-main">
                            <div class="run-detail-head">
                              <strong>{item.sonarr_title}</strong>
                              <Badge type={itemStatusType(item.status)} label={item.status} />
                            </div>
                            <p>{item.summary}</p>
                          </div>
                          <div class="run-detail-meta">
                            <span>{item.episodes_changed} changed</span>
                          </div>
                          {#if item.error}
                            <div class="run-detail-error">
                              <IconWarningCircleRegular size={15} weight="regular" />
                              <span>{item.error}</span>
                            </div>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="run-detail-empty">No anime-level summary was recorded for this run.</div>
                  {/if}

                  {#if log.errors}
                    <div class="activity-errors">
                      <div class="error-head"><IconWarningCircleRegular size={16} weight="regular" /> Run errors</div>
                      {log.errors}
                    </div>
                  {/if}
                </div>
              {/if}
            </article>
          {/each}
        </div>

        {#if hasMore}
          <div class="load-more-row">
            <button class="btn btn-secondary" on:click={() => loadActivity(false)} disabled={loadingMore}>
              <span>{loadingMore ? 'Loading...' : 'Load more'}</span>
            </button>
          </div>
        {/if}
      </section>
    {/if}
  {/if}
</div>

<style>
  .activity-shell {
    display: grid;
    gap: 18px;
    max-width: 1160px;
    margin: 0 auto;
  }

  .compact-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
  }

  .compact-heading h1 {
    margin-bottom: 4px;
    font-size: clamp(1.75rem, 2vw, 2.05rem);
  }

  .compact-heading p {
    color: var(--text-secondary);
  }

  .compact-heading .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .compact-strip {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }

  .mini-stat {
    display: grid;
    gap: 5px;
    padding: 15px 17px;
    background: rgba(11, 18, 31, 0.78);
  }

  .mini-label {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .mini-stat strong {
    font-family: 'Figtree', sans-serif;
    font-size: 1.02rem;
  }

  .mini-stat small {
    color: var(--text-secondary);
  }

  .state-online {
    color: var(--success);
  }

  .state-offline {
    color: var(--danger);
  }

  .concise-panel {
    padding: 0;
    overflow: hidden;
  }

  .aligned-panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 16px 18px 14px;
    border-bottom: 1px solid rgba(118, 167, 203, 0.12);
    background: linear-gradient(180deg, rgba(9, 18, 30, 0.72), rgba(9, 18, 30, 0.28));
  }

  .aligned-panel-head h3 {
    margin-bottom: 4px;
    font-size: 1rem;
  }

  .aligned-panel-head p {
    color: var(--text-secondary);
    font-size: 13px;
  }

  .run-list {
    display: grid;
  }

  .run-item {
    border-bottom: 1px solid rgba(118, 167, 203, 0.1);
  }

  .run-summary {
    width: 100%;
    display: grid;
    grid-template-columns: 42px minmax(220px, 1.6fr) repeat(4, minmax(110px, 0.8fr)) auto;
    align-items: center;
    gap: 14px;
    padding: 14px 18px;
    text-align: left;
    background: transparent;
    color: inherit;
  }

  .run-item:hover .run-summary {
    background: rgba(18, 168, 216, 0.04);
  }

  .run-item.expanded .run-summary {
    background: rgba(14, 27, 45, 0.44);
  }

  .run-expand {
    width: 30px;
    height: 30px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 999px;
    border: 1px solid rgba(118, 167, 203, 0.14);
    background: rgba(10, 18, 30, 0.82);
    color: var(--text-secondary);
    transition: transform var(--transition), color var(--transition), border-color var(--transition);
  }

  .run-item.expanded .run-expand {
    transform: rotate(90deg);
    color: var(--text-primary);
    border-color: rgba(118, 167, 203, 0.22);
  }

  .run-col,
  .run-status-col {
    display: grid;
    gap: 4px;
    min-width: 0;
  }

  .run-label {
    color: #88a8c3;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .run-col strong {
    font-size: 13px;
    color: var(--text-primary);
  }

  .run-status-col {
    justify-items: end;
  }

  .run-detail {
    display: grid;
    gap: 14px;
    padding: 0 18px 18px 60px;
    background: rgba(8, 16, 27, 0.56);
  }

  .run-detail-list {
    display: grid;
    gap: 10px;
  }

  .run-detail-item {
    display: grid;
    gap: 10px;
    padding: 12px 14px;
    border-radius: 16px;
    border: 1px solid rgba(118, 167, 203, 0.12);
    background: rgba(255, 255, 255, 0.03);
  }

  .run-detail-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .run-detail-main p {
    margin-top: 6px;
    color: var(--text-secondary);
    font-size: 13px;
  }

  .run-detail-meta {
    color: #88a8c3;
    font-size: 12px;
  }

  .run-detail-error {
    display: inline-flex;
    align-items: flex-start;
    gap: 8px;
    color: #fecaca;
    font-size: 12px;
  }

  .run-detail-empty {
    color: var(--text-secondary);
    font-size: 13px;
  }

  .activity-errors {
    padding: 12px 14px;
    border-radius: 14px;
    border: 1px solid rgba(239, 68, 68, 0.25);
    background: rgba(95, 24, 32, 0.32);
    color: #fecaca;
    font-family: Consolas, 'SFMono-Regular', monospace;
    font-size: 12px;
    white-space: pre-wrap;
  }

  .error-head {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 8px;
    font-family: 'Figtree', sans-serif;
    color: #fecaca;
  }

  .load-more-row {
    display: flex;
    justify-content: center;
    padding: 16px 18px 18px;
  }

  .activity-empty {
    min-height: 240px;
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

  .compact-empty .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  @media (max-width: 1100px) {
    .run-summary {
      grid-template-columns: 42px minmax(180px, 1.4fr) repeat(2, minmax(100px, 0.7fr)) auto auto;
    }

    .run-summary .run-col:nth-child(5),
    .run-summary .run-col:nth-child(6) {
      display: none;
    }
  }

  @media (max-width: 900px) {
    .compact-heading {
      flex-direction: column;
      align-items: flex-start;
    }

    .compact-strip {
      grid-template-columns: 1fr;
    }

    .run-summary {
      grid-template-columns: 42px 1fr auto;
      gap: 12px;
    }

    .run-started {
      min-width: 0;
    }

    .run-summary .run-col:not(.run-started),
    .run-status-col {
      display: none;
    }

    .run-detail {
      padding-left: 18px;
    }
  }
</style>
