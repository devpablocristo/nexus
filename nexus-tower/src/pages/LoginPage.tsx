import { SignIn } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

export default function LoginPage() {
  if (!clerkEnabled) {
    return (
      <div className="panel-page">
        <h2>Login disabled</h2>
        <p className="muted">Set `VITE_CLERK_PUBLISHABLE_KEY` to enable Clerk authentication.</p>
      </div>
    );
  }
  return (
    <div className="auth-page">
      <SignIn routing="path" path="/login" signUpUrl="/signup" />
    </div>
  );
}

