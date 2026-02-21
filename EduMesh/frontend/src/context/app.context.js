import { createContext, useCallback, useContext, useMemo, useState } from "react";

const STORAGE_KEY = "edumesh_state";
const AppContext = createContext();

const defaultState = {
  role: "",
  classroomId: "",
  classroomTitle: "",
  classroomTeacher: "",
  autoApprove: true,
};

function loadState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultState;
    return { ...defaultState, ...JSON.parse(raw) };
  } catch {
    return defaultState;
  }
}

export const AppProvider = ({ children }) => {
  const [state, setState] = useState(loadState);

  const persist = useCallback((producer) => {
    setState((prev) => {
      const next = typeof producer === "function" ? producer(prev) : producer;
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
      return next;
    });
  }, []);

  const setRole = useCallback((role) => {
    persist((prev) => ({ ...prev, role }));
  }, [persist]);
  const setClassroom = useCallback((id, title, teacher) => {
    persist((prev) => ({
      ...prev,
      classroomId: id || "",
      classroomTitle: title || "",
      classroomTeacher: teacher || "",
    }));
  }, [persist]);
  const clearClassroom = useCallback(() => {
    persist((prev) => ({
      ...prev,
      classroomId: "",
      classroomTitle: "",
      classroomTeacher: "",
    }));
  }, [persist]);
  const setAutoApprove = useCallback((autoApprove) => {
    persist((prev) => ({ ...prev, autoApprove }));
  }, [persist]);

  const value = useMemo(
    () => ({
      ...state,
      setRole,
      setClassroom,
      clearClassroom,
      setAutoApprove,
    }),
    [state, setRole, setClassroom, clearClassroom, setAutoApprove]
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
};

export const useApp = () => useContext(AppContext);
