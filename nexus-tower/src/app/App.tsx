import { useQuery } from '@tanstack/react-query';
import { Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom';

import { ToolProvider } from '../lib/tool-context';
import { Shell } from '../components/Shell';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ToolsPage } from '../features/tools/ToolsPage';
import { AuditPage } from '../features/audit/AuditPage';
import { MonitoringPage } from '../features/monitoring/MonitoringPage';
import { clerkEnabled } from '../lib/auth';
import LoginPage from '../pages/LoginPage';
import SignupPage from '../pages/SignupPage';
import SettingsPage from '../pages/SettingsPage';
import OrganizationSettingsPage from '../pages/OrganizationSettingsPage';
import OrgSelectorPage from '../pages/OrgSelectorPage';
import APIKeysPage from '../pages/APIKeysPage';
import SecretsPage from '../pages/SecretsPage';
import PoliciesPage from '../pages/PoliciesPage';
import IncidentsPage from '../pages/IncidentsPage';
import EventsPage from '../pages/EventsPage';
import AssistantPage from '../pages/AssistantPage';
import BillingPage from '../pages/BillingPage';
import BillingSuccessPage from '../pages/BillingSuccessPage';
import AdminPage from '../pages/AdminPage';
import AdminActivityPage from '../pages/AdminActivityPage';
import NotificationPreferencesPage from '../pages/NotificationPreferencesPage';
import DeveloperPage from '../pages/DeveloperPage';
import NotFoundPage from '../pages/NotFoundPage';
import SuspendedPage from '../pages/SuspendedPage';
import OnboardingPage from '../pages/OnboardingPage';
import NotificationsPage from '../pages/NotificationsPage';
import { getTools } from '../lib/api';

function OnboardingGuard() {
  const location = useLocation();
  const toolsQuery = useQuery({ queryKey: ['tools'], queryFn: getTools });
  const isOnboardingRoute = location.pathname.startsWith('/onboarding');
  const isSuspendedRoute = location.pathname.startsWith('/suspended');
  const toolsCount = toolsQuery.data?.items.length ?? 0;

  if (toolsQuery.isLoading || toolsQuery.isError || isSuspendedRoute) {
    return <Outlet />;
  }
  if (toolsCount === 0 && !isOnboardingRoute) {
    return <Navigate to="/onboarding" replace />;
  }
  if (toolsCount > 0 && isOnboardingRoute) {
    return <Navigate to="/tools" replace />;
  }
  return <Outlet />;
}

function ProtectedLayout() {
  return (
    <ProtectedRoute>
      <ToolProvider>
        <Shell>
          <OnboardingGuard />
        </Shell>
      </ToolProvider>
    </ProtectedRoute>
  );
}

export function App() {
  return (
    <>
      {clerkEnabled && <AuthTokenBridge />}
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />

        <Route element={<ProtectedLayout />}>
          <Route path="/" element={<Navigate to="/tools" replace />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/monitoring" element={<MonitoringPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/settings/org" element={<OrganizationSettingsPage />} />
          <Route path="/org-selector" element={<OrgSelectorPage />} />
          <Route path="/settings/keys" element={<APIKeysPage />} />
          <Route path="/secrets" element={<SecretsPage />} />
          <Route path="/policies" element={<PoliciesPage />} />
          <Route path="/incidents" element={<IncidentsPage />} />
          <Route path="/events" element={<EventsPage />} />
          <Route path="/assistant" element={<AssistantPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/admin/activity" element={<AdminActivityPage />} />
          <Route path="/developer" element={<DeveloperPage />} />
          <Route path="/billing" element={<BillingPage />} />
          <Route path="/billing/success" element={<BillingSuccessPage />} />
          <Route path="/settings/notifications" element={<NotificationPreferencesPage />} />
          <Route path="/notifications" element={<NotificationsPage />} />
          <Route path="/onboarding" element={<OnboardingPage />} />
          <Route path="/suspended" element={<SuspendedPage />} />
        </Route>

        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </>
  );
}
