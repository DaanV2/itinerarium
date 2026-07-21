<script lang="ts">
	/** A trigger button that opens a Modal containing a create form. Replaces the
	 * always-visible "create" sections that used to sit on top of list pages.
	 *
	 * The caller supplies the form fields via `children` and does the actual work
	 * in `onSubmit` (call the API, then reset the bound field values). onSubmit
	 * should throw on failure — the error is caught and shown inside the modal.
	 *
	 * "Create another" keeps the modal open after a successful create so the user
	 * can add several in a row; unchecked, a successful create closes it. */
	import type { Snippet } from 'svelte';
	import ErrorAlert from './ErrorAlert.svelte';
	import Modal from './Modal.svelte';
	import SubmitButton from './SubmitButton.svelte';

	let {
		triggerLabel,
		title = triggerLabel,
		submitLabel = triggerLabel,
		pendingLabel = 'Saving…',
		onSubmit,
		children
	}: {
		triggerLabel: string;
		/** Modal heading. Defaults to the trigger label. */
		title?: string;
		/** Submit button text. Defaults to the trigger label. */
		submitLabel?: string;
		pendingLabel?: string;
		/** Does the create; throws on failure. Reset field values here on success. */
		onSubmit: () => Promise<void>;
		children: Snippet;
	} = $props();

	let open = $state(false);
	let submitting = $state(false);
	let error = $state('');
	let createAnother = $state(false);

	function openModal() {
		error = '';
		open = true;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
		try {
			await onSubmit();
			if (!createAnother) {
				open = false;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong.';
		} finally {
			submitting = false;
		}
	}
</script>

<button type="button" onclick={openModal}>{triggerLabel}</button>

<Modal bind:open {title}>
	<form onsubmit={handleSubmit}>
		<ErrorAlert message={error} />

		{@render children()}

		<div class="footer">
			<label class="create-another">
				<input type="checkbox" bind:checked={createAnother} />
				Create another
			</label>
			<SubmitButton pending={submitting} label={submitLabel} {pendingLabel} />
		</div>
	</form>
</Modal>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-top: 1rem;
	}

	.create-another {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.9rem;
	}
</style>
