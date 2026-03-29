<script>
  import { Button as FoxButton, cn } from '@foxui/core';

  let className = '';
  export { className as class };

  export let variant = 'primary';
  export let size = 'md';
  export let block = false;

  const foxVariantMap = {
    primary: 'primary',
    secondary: 'secondary',
    danger: 'red',
    ghost: 'ghost',
    link: 'link',
  };

  const foxSizeMap = {
    sm: 'sm',
    md: 'default',
    lg: 'lg',
    icon: 'icon',
  };

  $: foxVariant = foxVariantMap[variant] ?? 'primary';
  $: foxSize = foxSizeMap[size] ?? 'default';
</script>

<FoxButton
  variant={foxVariant}
  size={foxSize}
  class={cn('ui-button', `ui-button-${variant}`, `ui-button-${size}`, block && 'ui-button-block', className)}
  {...$$restProps}
>
  <slot />
</FoxButton>
