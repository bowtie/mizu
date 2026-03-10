<script>
  export let type = 'unknown'; // canon, mixed_canon, anime_canon, filler, special, unknown
  export let label = '';

  const typeMap = {
    canon: { class: 'badge-canon', text: 'Manga' },
    mixed_canon: { class: 'badge-mixed', text: 'Mixed' },
    mixed: { class: 'badge-mixed', text: 'Mixed' },
    anime_canon: { class: 'badge-anime-canon', text: 'Anime' },
    filler: { class: 'badge-filler', text: 'Filler' },
    special: { class: 'badge-special', text: 'Special' },
    unknown: { class: 'badge-unknown', text: 'Unknown' },
    success: { class: 'badge-success', text: 'Success' },
    warning: { class: 'badge-warning', text: 'Warning' },
    danger: { class: 'badge-danger', text: 'Danger' },
    error: { class: 'badge-danger', text: 'Error' },
    info: { class: 'badge-info', text: 'Info' },
    partial: { class: 'badge-warning', text: 'Partial' },
    failed: { class: 'badge-danger', text: 'Failed' },
    running: { class: 'badge-info', text: 'Running' },
  };

  $: info = typeMap[type] || typeMap.unknown;
  $: displayText = label || info.text;
</script>

<span class="badge {info.class}">{displayText}</span>




