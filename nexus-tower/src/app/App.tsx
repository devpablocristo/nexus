import { Route, Routes } from 'react-router-dom';

import { Shell } from '../components/Shell';
import { ViewerPage } from '../features/viewer/ViewerPage';
import { AskAgentPage } from '../features/ask-agent/AskAgentPage';
import { ExportsPage } from '../features/exports/ExportsPage';
import { OverviewPage } from '../features/overview/OverviewPage';
import { PoliciesPage } from '../features/policies/PoliciesPage';
import { RunExplorerPage } from '../features/run-explorer/RunExplorerPage';
import { TimelinePage } from '../features/timeline/TimelinePage';
import { ApprovalsPage } from '../features/approvals/ApprovalsPage';
import { AlertsPage } from '../features/alerts/AlertsPage';
import { SessionsPage } from '../features/sessions/SessionsPage';

export function App() {
  return (
    <Shell>
      <Routes>
        <Route path="/" element={<OverviewPage />} />
        <Route path="/viewer" element={<ViewerPage />} />
        <Route path="/run-explorer" element={<RunExplorerPage />} />
        <Route path="/timeline" element={<TimelinePage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/approvals" element={<ApprovalsPage />} />
        <Route path="/alerts" element={<AlertsPage />} />
        <Route path="/sessions" element={<SessionsPage />} />
        <Route path="/ask-agent" element={<AskAgentPage />} />
        <Route path="/exports" element={<ExportsPage />} />
      </Routes>
    </Shell>
  );
}
