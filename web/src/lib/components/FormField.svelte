<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	// A labelled text/email/password/number input. Any extra attribute (type,
	// required, autocomplete, minlength, min…) is forwarded to the <input>.
	interface Props extends HTMLInputAttributes {
		id: string;
		label: string;
		value: string | number;
	}

	let { id, label, value = $bindable(), ...rest }: Props = $props();
</script>

<label for={id}>{label}</label>
<input {id} bind:value {...rest} />
