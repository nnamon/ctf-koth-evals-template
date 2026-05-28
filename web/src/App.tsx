import { BrowserRouter, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AuthProvider, useAuth, useAuthValue } from "./auth";
import { Login } from "./pages/Login";
import { Overview } from "./pages/Overview";
import { NewSuite } from "./pages/NewSuite";
import { SuiteDetail } from "./pages/SuiteDetail";
import { Submit } from "./pages/Submit";
import { Submissions } from "./pages/Submissions";
import { SubmissionDetail } from "./pages/SubmissionDetail";
import { RunDetail } from "./pages/RunDetail";

function App() {
  const auth = useAuthValue();
  return (
    <AuthProvider value={auth}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <RequireAuth>
                <Overview />
              </RequireAuth>
            }
          />
          <Route
            path="/suites/new"
            element={
              <RequireAuth>
                <NewSuite />
              </RequireAuth>
            }
          />
          <Route
            path="/suites/:id"
            element={
              <RequireAuth>
                <SuiteDetail />
              </RequireAuth>
            }
          />
          <Route
            path="/submit"
            element={
              <RequireAuth>
                <Submit />
              </RequireAuth>
            }
          />
          <Route
            path="/submissions"
            element={
              <RequireAuth>
                <Submissions />
              </RequireAuth>
            }
          />
          <Route
            path="/submissions/:id"
            element={
              <RequireAuth>
                <SubmissionDetail />
              </RequireAuth>
            }
          />
          <Route
            path="/runs/:id"
            element={
              <RequireAuth>
                <RunDetail />
              </RequireAuth>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

function RequireAuth({ children }: { children: React.ReactElement }) {
  const { authenticated } = useAuth();
  const location = useLocation();
  if (!authenticated) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }
  return children;
}

export default App;
