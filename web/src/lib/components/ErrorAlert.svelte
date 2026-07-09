<script lang="ts">
	// Renders an accessible error message, or nothing when `message` is empty.
	let { message }: { message: string } = $props();
</script>

{#if message}
	<p role="alert">{message}</p>
{/if}
