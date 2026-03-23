import { resolveClerkBrowserConfig } from '@devpablocristo/core-authn/providers/clerk'

const clerkConfig = resolveClerkBrowserConfig()

export const clerkEnabled = clerkConfig.enabled
export const clerkPublishableKey = clerkConfig.publishableKey

// API key fallback para desarrollo sin Clerk
export const REVIEW_API_KEY = 'nexus-review-admin-dev-key'
export const COMPANION_API_KEY = 'nexus-companion-admin-dev-key'
