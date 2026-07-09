<script lang="ts">
	// Submit button that disables itself and swaps its label while a request is
	// in flight. `label` is the idle text; `pendingLabel` is shown while `pending`.
	let {
		pending = false,
		label,
		pendingLabel
	}: { pending?: boolean; label: string; pendingLabel: string } = $props();
</script>

<button type="submit" disabled={pending}>
	{pending ? pendingLabel : label}
</button>
