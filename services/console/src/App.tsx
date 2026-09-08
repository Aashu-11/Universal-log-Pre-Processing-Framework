import { Navigate, Route, Routes } from "react-router-dom";

import { AppShell } from "./components/layout/AppShell";
import { useAuth } from "./lib/auth";
import { Login } from "./pages/Login";
import { LiveTheater } from "./pages/LiveTheater";
import { Pipeline } from "./pages/Pipeline";
import { Sources } from "./pages/Sources";
import { Explorer } from "./pages/Explorer";
import { Traceability } from "./pages/Traceability";
import { ParserWorkbench } from "./pages/ParserWorkbench";
import { DLQ } from "./pages/DLQ";
import { ReviewerMode } from "./pages/ReviewerMode";

function ProtectedShell() {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<LiveTheater />} />
        <Route path="/pipeline" element={<Pipeline />} />
        <Route path="/sources" element={<Sources />} />
        <Route path="/explorer" element={<Explorer />} />
        <Route path="/traceability" element={<Traceability />} />
        <Route path="/workbench" element={<ParserWorkbench />} />
        <Route path="/dlq" element={<DLQ />} />
        <Route path="/reviewer" element={<ReviewerMode />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/*" element={<ProtectedShell />} />
    </Routes>
  );
}
