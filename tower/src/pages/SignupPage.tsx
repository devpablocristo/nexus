import { SignUp } from '@clerk/clerk-react';

import { clerkEnabled } from '../lib/auth';

export default function SignupPage() {
  if (!clerkEnabled) {
    return (
      <div className="panel-page">
        <h2>Signup disabled</h2>
        <p className="muted">Set `VITE_CLERK_PUBLISHABLE_KEY` to enable Clerk authentication.</p>
      </div>
    );
  }
  return (
    <div className="auth-page">
      <SignUp routing="path" path="/signup" signInUrl="/login" />
    </div>
  );
}

