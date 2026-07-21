<script lang="ts">
	/** Generic modal dialog. Caller supplies the content via `children` (and an
	 * optional `footer` snippet for action buttons). Built on the native
	 * <dialog> element to stay dependency-free — same convention as
	 * ConcurrentEditDialog. Backdrop click and Esc both close it; `open` is
	 * bindable so callers drive it and stay in sync when the user dismisses. */
	import type { Snippet } from 'svelte';

	let {
		open = $bindable(false),
		title,
		onClose,
		children,
		footer
	}: {
		open?: boolean;
		title: string;
		onClose?: () => void;
		children: Snippet;
		footer?: Snippet;
	} = $props();

	let dialogEl: HTMLDialogElement | undefined = $state();

	// Sync the DOM element to `open`. Guard on dialogEl.open so we never call
	// showModal() on an already-open dialog (throws) or close() on a closed one.
	$effect(() => {
		if (open && !dialogEl?.open) {
			dialogEl?.showModal();
		} else if (!open && dialogEl?.open) {
			dialogEl?.close();
		}
	});

	// Fires on Esc (native), backdrop click, and the close button. The `open`
	// guard makes it a no-op when we closed programmatically, so onClose runs
	// exactly once per user dismissal.
	function requestClose() {
		if (!open) return;
		open = false;
		onClose?.();
	}
</script>

<dialog
	bind:this={dialogEl}
	onclose={requestClose}
	onclick={(e) => e.target === dialogEl && requestClose()}
>
	<div class="head">
		<h2>{title}</h2>
		<button type="button" class="close" aria-label="Close" onclick={requestClose}>×</button>
	</div>

	<div class="body">
		{@render children()}
	</div>

	{#if footer}
		<div class="actions">
			{@render footer()}
		</div>
	{/if}
</dialog>

<style>
	dialog {
		border: none;
		border-radius: 8px;
		padding: 1.25rem;
		width: 28rem;
		max-width: calc(100vw - 2rem);
	}

	dialog::backdrop {
		background: rgba(0, 0, 0, 0.4);
	}

	.head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.head h2 {
		margin: 0;
		font-size: 1.25rem;
	}

	.close {
		border: none;
		background: none;
		font-size: 1.5rem;
		line-height: 1;
		cursor: pointer;
		padding: 0 0.25rem;
		color: inherit;
	}

	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1rem;
	}
</style>
