import { createContext, useEffect, useState, useContext } from "react";

const ProfileContext = createContext();
const STORAGE_KEY = "rdp_profile";
export const ProfileProvider = ({ children }) => {
  const [profile, setProfile] = useState(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  });
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    setIsLoading(false);
  }, []);

  const signIn = (name) => {
    if (!name) return;
    const data = {
      name,
      email: name,
      avatar: "",
    };
    setProfile(data);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
  };

  const signOut = () => {
    setProfile(null);
    localStorage.removeItem(STORAGE_KEY);
  };

  return (
    <ProfileContext.Provider value={{ profile, isLoading, signIn, signOut }}>
      {children}
    </ProfileContext.Provider>
  );
};
export const useProfile = () => {
  return useContext(ProfileContext);
};
