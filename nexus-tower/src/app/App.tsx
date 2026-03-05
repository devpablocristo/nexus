import { Navigate, Outlet, Route, Routes } from 'react-router-dom';

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

function ProtectedLayout() {
  return (
    <ProtectedRoute>
      <ToolProvider>
        <Shell>
          <Outlet />
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
          <Route path="/billing" element={<BillingPage />} />
          <Route path="/billing/success" element={<BillingSuccessPage />} />
        </Route>

        <Route path="*" element={<Navigate to="/tools" replace />} />
      </Routes>
    </>
  );
}
