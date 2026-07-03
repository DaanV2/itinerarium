// TypeScript types mirroring the Go API models (infrastructure/persistence/models).

export interface SetupStatus {
	needs_setup: boolean;
}

export interface InitialAccount {
	id: string;
	email: string;
	access_token: string;
}
