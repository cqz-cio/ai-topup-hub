export interface ExternalIdentityUnbindCapability {
  can_unbind?: unknown
}

/**
 * Unbinding is a backend-authorized operation. Missing, stale, or malformed
 * capability data must never enable an action that could lock the user out.
 */
export const canUnbindExternalIdentity = (
  binding: ExternalIdentityUnbindCapability | null | undefined,
): boolean => binding?.can_unbind === true
