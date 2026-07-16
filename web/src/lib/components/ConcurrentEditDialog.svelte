<script lang="ts">
	/** Warns before a save would overwrite someone else's concurrent edit
	 * (core domain rule 7) — shown when the API returns 409 `concurrent_edit`.
	 * Native <dialog> keeps this dependency-free per the repo's minimal-UI
	 * convention. */
	let {
		open,
		onReload,
		onOverwrite,
		onCancel
	}: {
		open: boolean;
		onReload: () => void;
		onOverwrite: () => void;
		onCancel: () => void;
	} = $props();

	let dialogEl: HTMLDialogElement | undefined = $state();

	$effect(() => {
		if (open) {
			dialogEl?.showModal();
		} else {
			dialogEl?.close();
		}
	});
</script>

<dialog
	bind:this={dialogEl}
	onclose={onCancel}
	onclick={(e) => e.target === dialogEl && onCancel()}
>
	<h2>Someone else edited this document</h2>
	<p>
		This document changed since you started editing. Reload to see the latest version and redo your
		changes, or overwrite it with what you have.
	</p>
	<div class="actions">
		<button type="button" onclick={onCancel}>Keep editing</button>
		<button type="button" onclick={onReload}>Reload latest</button>
		<button type="button" class="danger" onclick={onOverwrite}>Overwrite anyway</button>
	</div>
</dialog>

<style>
	dialog {
		border: none;
		border-radius: 8px;
		padding: 1.25rem;
		max-width: 28rem;
	}

	dialog::backdrop {
		background: rgba(0, 0, 0, 0.4);
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1rem;
	}

	.danger {
		background-color: rgba(220, 38, 38, 0.9);
		color: white;
		border: none;
	}
</style>
