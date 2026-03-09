<script>
  import { onMount, onDestroy } from 'svelte';
  import { addNotification } from '../lib/stores.js';
  import { getSyncLogs } from '../lib/api.js';
  import Spinner from '../components/Spinner.svelte';
  import Badge from '../components/Badge.svelte';

  let loading = true;
  let logs = [];
  let expandedId = null;
  let autoRefresh = false;
  let refreshInterval = null;

  onMount(async () => {
    await loadLogs();
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  async function loadLogs() {
    loading = logs.length === 0;
    try {
      logs = await getSyncLogs() || [];
    } catch (err) {
      addNotification('Failed to load sync logs: ' + err.message, 'error');
    }
    loading = false;
  }

  function toggleAutoRefresh() {
    autoRefresh = !autoRefresh;
    if (autoRefresh) {
      refreshInterval = setInterval(loadLogs, 5000);
      addNotification('Auto-refresh enabled (5s)', 'info', 2000);
    } else {
      if (refreshInterval) clearInterval(refreshInterval);
      refreshInterval = null;
    }
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id;
  }

  function formatDuration(startedAt, completedAt) {
    if (!startedAt || !completedAt) return '-';
    const ms = new Date(completedAt) - new Date(startedAt);
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    return Math.floor(ms / 60000) + 'm ' + Math.round((ms % 60000) / 1000) + 's';
  }

  function formatTime(dt) {
    if (!dt) return '-';
    return new Date(dt).toLocaleString();
  }

  function statusType(status) {
    switch (status) {
      case 'success': return 'success';
      case 'partial': return 'warning';
      case 'failed': return 'failed';
      case 'running': return 'running';
      default: return 'unknown';
    }
  }
</script>

<div class="page-header">
  <div>
    <h2>Sync Logs</h2>
    <p>{logs.length} sync run{logs.length === 1 ? '' : 's'} recorded</p>
  </div>
  <div class="btn-group">
    <button
      class="btn btn-sm {autoRefresh ? 'btn-primary' : 'btn-secondary'}"
      on:click={toggleAutoRefresh}
    >
      {autoRefresh ? '&#9210; Auto-Refresh ON' : '&#9210; Auto-Refresh OFF'}
    </button>
    <button class="btn btn-sm btn-secondary" on:click={loadLogs}>
      &#8635; Refresh
    </button>
  </div>
</div>

{#if loading}
  <Spinner center size="lg" message="Loading logs..." />
{:else if logs.length === 0}
  <div class="card">
    <div class="empty-state">
      <div class="empty-icon">&#128220;</div>
      <h3>No Sync Logs</h3>
      <p>Sync logs will appear here after your first sync run. Trigger a sync from the Dashboard.</p>
      <a href="#/" class="btn btn-primary">Go to Dashboard</a>
    </div>
  </div>
{:else}
  <div class="table-container">
    <table>
      <thead>
        <tr>
          <th style="width:30px"></th>
          <th>Started</th>
          <th>Duration</th>
          <th>Shows Processed</th>
          <th>Episodes Changed</th>
          <th>Trigger</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {#each logs as log}
          <tr>
            <td>
              <button
                class="expand-toggle"
                class:expanded={expandedId === log.id}
                on:click={() => toggleExpand(log.id)}
              >
                &#9654;
              </button>
            </td>
            <td>{formatTime(log.started_at)}</td>
            <td>{formatDuration(log.started_at, log.completed_at)}</td>
            <td>{log.shows_processed}</td>
            <td>{log.episodes_updated}</td>
            <td>
              <span style="text-transform:capitalize;font-size:12px;color:var(--text-secondary)">
                {log.trigger || log.trigger_type || '-'}
              </span>
            </td>
            <td><Badge type={statusType(log.status)} label={log.status} /></td>
          </tr>

          {#if expandedId === log.id}
            <tr class="expanded-content">
              <td colspan="7">
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;max-width:600px">
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Started At</div>
                    <div style="font-size:13px">{formatTime(log.started_at)}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Completed At</div>
                    <div style="font-size:13px">{formatTime(log.completed_at)}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Shows Processed</div>
                    <div style="font-size:13px">{log.shows_processed}</div>
                  </div>
                  <div>
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Episodes Updated</div>
                    <div style="font-size:13px">{log.episodes_updated}</div>
                  </div>
                </div>
                {#if log.errors}
                  <div style="margin-top:12px">
                    <div style="font-size:11px;color:var(--text-muted);text-transform:uppercase;margin-bottom:4px">Errors</div>
                    <div style="font-size:12px;color:var(--danger);background:var(--danger-bg);padding:8px 12px;border-radius:var(--radius);font-family:monospace;white-space:pre-wrap">{log.errors}</div>
                  </div>
                {/if}
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}

