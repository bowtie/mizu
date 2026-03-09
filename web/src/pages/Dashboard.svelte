<script>
  import { onMount } from 'svelte';
  import { shows, syncStatus, addNotification } from '../lib/stores.js';
  import { getStatus, getStats, getShows, getSyncLogs, triggerSync, getVersion } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';

  let loading = true;
  let stats = { canon: 0, mixed: 0, filler: 0, total: 0 };
  let recentLogs = [];
  let version = '';
  let syncing = false;
  let sonarrConnected = false;
  let aflReachable = false;
  let mappedCount = 0;

  onMount(async () => {
    await loadDashboard();
  });

  async function loadDashboard() {
    loading = true;
    try {
      const [statusRes, statsRes, showsRes, logsRes, versionRes] = await Promise.allSettled([
        getStatus(),
        getStats(),
        getShows(),
        getSyncLogs(),
        getVersion(),
      ]);

      if (statusRes.status === 'fulfilled') {
        sonarrConnected = statusRes.value.status === 'ok';
        aflReachable = true;
        syncStatus.update(s => ({ ...s, connected: sonarrConnected, aflReachable: true }));
      }

      if (statsRes.status === 'fulfilled') {
        stats = statsRes.value;
      }

      if (showsRes.status === 'fulfilled') {
        const allShows = showsRes.value || [];
        shows.set(allShows);
        mappedCount = allShows.length;
      }

      if (logsRes.status === 'fulfilled') {
        recentLogs = (logsRes.value || []).slice(0, 5);
      }

      if (versionRes.status === 'fulfilled') {
        version = versionRes.value.version || '';
      }
    } catch (err) {
      addNotification('Failed to load dashboard data', 'error');
    }
    loading = false;
  }

  async function handleSync() {
    syncing = true;
    syncStatus.update(s => ({ ...s, syncing: true }));
    try {
      await triggerSync();
      addNotification('Sync triggered successfully', 'success');
      setTimeout(loadDashboard, 2000);
    } catch (err) {
      addNotification('Failed to trigger sync: ' + err.message, 'error');
    }
    syncing = false;
    syncStatus.update(s => ({ ...s, syncing: false }));
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return '-';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return ms + 'ms';
    return (ms / 1000).toFixed(1) + 's';
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }
</script>

{#if loading}
  <Spinner center size="lg" message="Loading dashboard..." />
{:else}
  <div class="page-header">
    <div>
      <h2>Dashboard</h2>
      <p>AnimeFillerGuard overview {version ? `v${version}` : ''}</p>
    </div>
    <button class="btn btn-primary" on:click={handleSync} disabled={syncing}>
      {#if syncing}
        <span class="spinner"></span> Syncing...
      {:else}
        &#8635; Sync Now
      {/if}
    </button>
  </div>

  <!-- Connection Status -->
  <div class="section">
    <div class="section-title">Connection Status</div>
    <div class="grid-2" style="max-width:500px">
      <div class="card" style="padding:14px 18px;display:flex;align-items:center;gap:10px">
        <span class="status-dot" class:online={sonarrConnected} class:offline={!sonarrConnected}></span>
        <div>
          <div style="font-size:13px;font-weight:600">{sonarrConnected ? 'Sonarr Connected' : 'Sonarr Offline'}</div>
          <div style="font-size:11px;color:var(--text-muted)">{sonarrConnected ? 'API responding' : 'Cannot reach API'}</div>
        </div>
      </div>
      <div class="card" style="padding:14px 18px;display:flex;align-items:center;gap:10px">
        <span class="status-dot" class:online={aflReachable} class:offline={!aflReachable}></span>
        <div>
          <div style="font-size:13px;font-weight:600">{aflReachable ? 'AFL Reachable' : 'AFL Unreachable'}</div>
          <div style="font-size:11px;color:var(--text-muted)">AnimeFillerList.com</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Stats Cards -->
  <div class="section">
    <div class="section-title">Overview</div>
    <div class="stats-grid">
      <div class="stat-card accent">
        <span class="stat-label">Mapped Shows</span>
        <span class="stat-value">{mappedCount}</span>
        <span class="stat-sub">Series tracked</span>
      </div>
      <div class="stat-card success">
        <span class="stat-label">Canon Episodes</span>
        <span class="stat-value">{stats.canon}</span>
        <span class="stat-sub">Monitored</span>
      </div>
      <div class="stat-card warning">
        <span class="stat-label">Mixed Episodes</span>
        <span class="stat-value">{stats.mixed}</span>
        <span class="stat-sub">Canon/Filler blend</span>
      </div>
      <div class="stat-card danger">
        <span class="stat-label">Filler Episodes</span>
        <span class="stat-value">{stats.filler}</span>
        <span class="stat-sub">Skipped / Unmonitored</span>
      </div>
    </div>
  </div>

  <!-- Recent Sync Logs -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Recent Syncs</h3>
        <a href="#/logs" class="btn btn-sm btn-secondary">View All</a>
      </div>
      {#if recentLogs.length === 0}
        <div class="empty-state" style="padding:24px">
          <p>No sync runs yet. Click "Sync Now" to start.</p>
        </div>
      {:else}
        <div class="table-container" style="border:none;border-radius:0">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Duration</th>
                <th>Shows</th>
                <th>Ep. Changed</th>
                <th>Trigger</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {#each recentLogs as log}
                <tr>
                  <td>{formatTime(log.started_at)}</td>
                  <td>{formatDuration(log.started_at, log.completed_at)}</td>
                  <td>{log.shows_processed}</td>
                  <td>{log.episodes_updated}</td>
                  <td style="text-transform:capitalize">{log.trigger || log.trigger_type || '-'}</td>
                  <td><Badge type={log.status} /></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>

  <!-- Quick Add -->
  <div class="section">
    <div class="card">
      <div class="card-header">
        <h3>Quick Add</h3>
        <a href="#/shows" class="btn btn-sm btn-secondary">Manage Shows</a>
      </div>
      <div style="padding:4px 0">
        {#if mappedCount === 0}
          <div class="empty-state" style="padding:20px">
            <div class="empty-icon">&#128253;</div>
            <h3>No Shows Mapped Yet</h3>
            <p>Head to the Shows page to map your Sonarr anime series to AnimeFillerList.</p>
            <a href="#/shows" class="btn btn-primary">Go to Shows</a>
          </div>
        {:else}
          <p style="font-size:13px;color:var(--text-secondary);padding:4px 0">
            {mappedCount} show{mappedCount === 1 ? '' : 's'} currently mapped and tracking filler data.
            Visit the Shows page to add more or manage existing mappings.
          </p>
        {/if}
      </div>
    </div>
  </div>
{/if}

