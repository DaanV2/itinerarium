import { redirect } from '@sveltejs/kit';
import { getSetupStatus } from '$lib/api/setup';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const status = await getSetupStatus(fetch);
	if (!status.needs_setup) {
		redirect(307, '/');
	}
};
