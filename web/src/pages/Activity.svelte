<script>
  import { onMount } from 'svelte';
  import { addNotification } from '../lib/stores.js';
  import { getSyncLogs, triggerSync } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';
  import { ChevronDown, History, RefreshCw, TriangleAlert } from 'lucide-svelte';

  const PAGE_SIZE = 12;

  let loading = true;
  let logs = [];
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
      const page = (await getSyncLogs(PAGE_SIZE, reset ? 0 : nextOffset)) || { items: [], has_more: false, next_offset: 0 };
      logs = reset ? (page.items || []) : [...logs, ...(page.items || [])];
      hasMore = !!page.has_more;
      nextOffset = page.next_offset || logs.length;
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

  function humanSummary(log) {
    const anime = log.shows_processed || 0;
    const episodes = log.episodes_updated || 0;
    return `Checked ${anime} anime, changed ${episodes} ${episodes === 1 ? 'episode' : 'episodes'}.`;
  }

  function detailLine(item) {
    if (item.summary) return item.summary;
    return item.episodes_changed > 0
      ? `${item.episodes_changed} ${item.episodes_changed === 1 ? 'episode change' : 'episode changes'}.`
      : 'No episode changes.';
  }

  function hasDetails(log) {
    return !!(log.items?.length || log.errors);
  }
</script>

<svelte:head>
  <title>Mizu - Activity</title>
</svelte:head>

<div class="activity-shell">
  <section class="page-heading activity-heading">
    <h1>Activity</h1>
    <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
      <RefreshCw size={18} />
      <span>{syncing ? 'Syncing...' : 'Sync now'}</span>
    </button>
  </section>

  {#if loading}
    <Spinner center size="lg" message="Surfacing recent activity..." />
  {:else if logs.length === 0}
    <div class="activity-empty">
      <div class="empty-state compact-empty">
        <div class="empty-icon"><History size={28} /></div>
        <h3>No sync activity yet</h3>
        <p>Run a sync to start building your history.</p>
        <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
          <RefreshCw size={18} />
          <span>Sync now</span>
        </button>
      </div>
    </div>
  {:else}
    <section class="activity-list">
      {#each logs as log}
        <article class="card run-card" class:expanded={expandedId === log.id}>
          <div class="run-card-head">
            <div class="run-card-meta">
              <Badge type={runStatusType(log.status)} label={log.status} />
              <span>{formatTime(log.started_at)}</span>
              <span>{formatTrigger(log.trigger)}</span>
              <span>{formatDuration(log.started_at, log.completed_at)}</span>
            </div>
            {#if hasDetails(log)}
              <button class="run-expand-btn" on:click={() => toggleExpand(log.id)} aria-label={expandedId === log.id ? 'Hide run details' : 'Show run details'}>
                <span>Details</span>
                <ChevronDown size={18} />
              </button>
            {/if}
          </div>

          <div class="run-card-body">
            <h3>{humanSummary(log)}</h3>
            {#if log.errors}
              <p>This run completed with errors that need attention.</p>
            {:else if log.items?.length}
              <p>Open details to see what changed for this run.</p>
            {/if}
          </div>

          {#if expandedId === log.id && hasDetails(log)}
            <div class="run-card-detail">
              {#if log.items?.length}
                <ul class="run-detail-list">
                  {#each log.items as item}
                    <li class="run-detail-item">
                      <div class="run-detail-head">
                        <strong>{item.sonarr_title}</strong>
                        <Badge type={itemStatusType(item.status)} label={item.status} />
                      </div>
                      <p>{detailLine(item)}</p>
                      {#if item.error}
                        <div class="run-detail-error">
                          <TriangleAlert size={18} />
                          <span>{item.error}</span>
                        </div>
                      {/if}
                    </li>
                  {/each}
                </ul>
              {/if}

              {#if log.errors}
                <div class="activity-errors">
                  <div class="error-head"><TriangleAlert size={18} /> Run errors</div>
                  {log.errors}
                </div>
              {/if}
            </div>
          {/if}
        </article>
      {/each}
    </section>

    {#if hasMore}
      <div class="load-more-row">
        <button class="btn btn-secondary" on:click={() => loadActivity(false)} disabled={loadingMore}>
          <span>{loadingMore ? 'Loading...' : 'Load more'}</span>
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .activity-shell {
    position: relative;
    isolation: isolate;
    display: grid;
    gap: var(--space-8);
    max-width: var(--content-max);
    margin: 0 auto;
  }

  .activity-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .activity-heading h1 {
    margin-bottom: 4px;
    font-size: var(--text-page-title);
    line-height: var(--leading-tight);
  }

  .activity-heading .btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .activity-list {
    display: grid;
    gap: 14px;
  }

  .run-card {
    padding: 16px 18px;
    display: grid;
    gap: 14px;
    background: var(--panel-bg-strong);
    border: none;
    box-shadow: var(--shadow-card);
  }

  .run-card-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
  }

  .run-card-meta {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    color: var(--text-secondary);
    font-size: var(--text-meta);
    line-height: 1.4;
  }

  .run-expand-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 0 10px;
    border-radius: 8px;
    border: 0;
    background: color-mix(in srgb, var(--surface-elevated) 82%, rgba(7, 16, 29, 0.96) 18%);
    color: var(--text-secondary);
    font-size: var(--text-meta);
    font-weight: var(--weight-semibold);
  }

  .run-card.expanded .run-expand-btn :global(svg) {
    transform: rotate(180deg);
  }

  .run-card-body {
    display: grid;
    gap: 6px;
  }

  .run-card-body h3 {
    font-size: var(--text-title-md);
    line-height: 1.25;
  }

  .run-card-body p {
    color: var(--text-secondary);
    font-size: var(--text-secondary-size);
    line-height: var(--leading-copy);
  }

  .run-card-detail {
    display: grid;
    gap: 10px;
    padding-top: 12px;
    box-shadow: inset 0 1px 0 var(--divider-border-soft);
  }

  .run-detail-list {
    display: grid;
    gap: 10px;
    list-style: none;
  }

  .run-detail-item {
    display: grid;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--panel-bg-soft);
    border: none;
  }

  .run-detail-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .run-detail-item p {
    color: var(--text-secondary);
    font-size: var(--text-secondary-size);
    line-height: var(--leading-copy);
  }

  .run-detail-error,
  .error-head {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .run-detail-error {
    color: var(--danger-foreground);
    font-size: var(--text-meta);
    line-height: 1.45;
  }

  .activity-errors {
    display: grid;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--danger) 10%, rgba(7, 16, 29, 0.92) 90%);
    box-shadow: inset 3px 0 0 rgba(255, 77, 106, 0.42);
    color: var(--danger-foreground);
    font-size: var(--text-secondary-size);
    line-height: var(--leading-copy);
  }

  .error-head {
    font-weight: 700;
    color: var(--danger-foreground);
  }

  .load-more-row {
    display: flex;
    justify-content: center;
  }

  .activity-empty {
    min-height: 240px;
    border: none;
    background: transparent;
    box-shadow: none;
  }

  .compact-empty {
    display: grid;
    gap: 10px;
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

  @media (max-width: 720px) {
    .activity-heading {
      flex-direction: column;
      align-items: stretch;
    }

    .activity-heading .btn {
      width: 100%;
    }

    .run-card {
      padding: 14px;
    }

    .run-card-head {
      flex-direction: column;
      align-items: stretch;
    }

    .run-card-meta {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
    }

    .run-expand-btn {
      width: 100%;
      justify-content: center;
    }

    .run-detail-head {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>






