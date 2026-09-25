import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";

import { AppShell } from "./components/layout/AppShell";
import { LoadingState } from "./components/ui/States";
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
import { InvestigationAssistant } from "./pages/InvestigationAssistant";

// Lazy-loaded: pulls in three.js/@react-three/fiber/@react-three/drei
// (~475KB gzipped), so this cost is only paid by someone who actually
// opens LogVerse, not on every console page load.
const LogVersePage = lazy(() => import("./pages/LogVersePage").then((m) => ({ default: m.LogVersePage })));

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
        <Route path="/assistant" element={<InvestigationAssistant />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}

function ProtectedLogVerse() {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <Suspense fallback={<LoadingState label="Loading LogVerse…" />}><LogVersePage /></Suspense>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/logverse" element={<ProtectedLogVerse />} />
      <Route path="/*" element={<ProtectedShell />} />
    </Routes>
  );
}
