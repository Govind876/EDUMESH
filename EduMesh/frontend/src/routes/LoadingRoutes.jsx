import { Outlet } from "react-router-dom";
import { useProfile } from "../context/profile.context";
import LoadingScreen from "../components/LoadingScreen";

const LoadingRoutes = () => {
  const { isLoading } = useProfile();
  return !isLoading ? <Outlet /> : <LoadingScreen />;
};
export default LoadingRoutes;
