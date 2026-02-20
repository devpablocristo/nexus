import { Route, Routes } from 'react-router-dom';

import { Shell } from '../components/Shell';
import { AskAgentPage } from '../features/ask-agent/AskAgentPage';
import { ExportsPage } from '../features/exports/ExportsPage';
import { OverviewPage } from '../features/overview/OverviewPage';
import { PoliciesPage } from '../features/policies/PoliciesPage';
import { TimelinePage } from '../features/timeline/TimelinePage';

export function App() {
  return (
    <Shell>
      <Routes>
        <Route path="/" element={<OverviewPage />} />
        <Route path="/timeline" element={<TimelinePage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/ask-agent" element={<AskAgentPage />} />
        <Route path="/exports" element={<ExportsPage />} />
      </Routes>
    </Shell>
  );
}
