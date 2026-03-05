# Grace Period & Dunning Policy

## Payment failure flow

1. Stripe fires `invoice.payment_failed` webhook
2. Nexus marks tenant as `past_due` immediately
3. Email `payment_failed` sent to org admins

## Grace period

- **Duration**: 14 days from first failed payment
- **During grace period**: Full API access maintained, banner shown in Tower
- **After grace period**: Tenant auto-suspended via scheduled job

## Auto-suspension

A worker in nexus-saas checks daily for tenants in `past_due` state
longer than 14 days and calls `SuspendTenant` automatically.

## Reactivation

- Update payment method in Stripe Customer Portal
- Stripe retries payment -> `invoice.paid` webhook
- Nexus auto-reactivates tenant
- OR: Admin manually reactivates via Admin Console

## Stripe retry schedule

Stripe Smart Retries handles payment retries (up to 4 attempts over ~3 weeks).
No custom retry logic needed on Nexus side.
