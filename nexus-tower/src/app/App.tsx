import { Navigate, Route, Routes } from 'react-router-dom';

import { ToolProvider } from '../lib/tool-context';
import { Shell } from '../components/Shell';
import { ToolsPage } from '../features/tools/ToolsPage';
import { AuditPage } from '../features/audit/AuditPage';
import { MonitoringPage } from '../features/monitoring/MonitoringPage';

export function App() {
  return (
    <ToolProvider>
      <Shell>
        <Routes>
          <Route path="/" element={<Navigate to="/tools" replace />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/monitoring" element={<MonitoringPage />} />
        </Routes>
      </Shell>
    </ToolProvider>
  );
}
