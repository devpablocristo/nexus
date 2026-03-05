import { RedirectToSignIn, useAuth } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

function LoadingSpinner() {
  return <p className="muted">Loading session...</p>;
}

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!clerkEnabled) {
    return <>{children}</>;
  }
  return <ClerkProtectedRoute>{children}</ClerkProtectedRoute>;
}

function ClerkProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isSignedIn, isLoaded } = useAuth();
  if (!isLoaded) return <LoadingSpinner />;
  if (!isSignedIn) return <RedirectToSignIn />;
  return <>{children}</>;
}
