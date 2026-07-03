// TypeScript types mirroring the Go API models (infrastructure/persistence/models).

export interface SetupStatus {
	needs_setup: boolean;
}

export interface InitialAccount {
	id: string;
	email: string;
	access_token: string;
}

export type Role = 'gm' | 'player';

export interface Account {
	id: string;
	email: string;
	role: Role;
}

export interface CreatedAccount extends Account {
	temporary_password: string;
}

export interface ResetPasswordResult {
	temporary_password: string;
}
