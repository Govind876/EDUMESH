import { Route, Routes, BrowserRouter, Navigate, useLocation } from "react-router-dom";
import { ProfileProvider } from "./context/profile.context";
import { AppProvider, useApp } from "./context/app.context";
import Splash from "./pages/Splash";
import RoleSelect from "./pages/RoleSelect";
import Discovery from "./pages/Discovery";
import Dashboard from "./pages/Dashboard";
import ContentViewer from "./pages/ContentViewer";
import Assignments from "./pages/Assignments";

const RequireRole = ({ children }) => {
  const { role } = useApp();
  const location = useLocation();
  if (!role) {
    return <Navigate to="/role" replace state={{ next: location.pathname + location.search }} />;
  }
  return children;
};

function App() {
  return (
    <BrowserRouter>
      <ProfileProvider>
        <AppProvider>
          <Routes>
            <Route path="/" element={<Splash />} />
            <Route path="/role" element={<RoleSelect />} />
            <Route
              path="/discover"
              element={
                <RequireRole>
                  <Discovery />
                </RequireRole>
              }
            />
            <Route
              path="/dashboard"
              element={
                <RequireRole>
                  <Dashboard />
                </RequireRole>
              }
            />
            <Route
              path="/content"
              element={
                <RequireRole>
                  <ContentViewer />
                </RequireRole>
              }
            />
            <Route
              path="/assignments"
              element={
                <RequireRole>
                  <Assignments />
                </RequireRole>
              }
            />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AppProvider>
      </ProfileProvider>
    </BrowserRouter>
  );
}

export default App;
