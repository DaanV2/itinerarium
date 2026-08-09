<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// A labelled text/email/password/number input. Any extra attribute (type,
	// required, autocomplete, minlength, min...) is forwarded to the <input>.
	interface Props extends HTMLInputAttributes {
		id: string;
		label: string;
		value: string | number;
	}

	let { id, label, value = $bindable(), ...rest }: Props = $props();
</script>

<div class="field">
	<label for={id}>{label}</label>
	<input {id} bind:value {...rest} />
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		margin-top: 0.75rem;
	}

	label {
		font-size: 0.875rem;
		font-weight: 500;
	}

	input {
		width: 100%;
	}
</style>
