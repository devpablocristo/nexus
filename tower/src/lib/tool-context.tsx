import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';

type ToolContextValue = {
  activeTool: string | null;
  setActiveTool: (name: string | null) => void;
};

const STORAGE_KEY = 'nexus_active_tool';

const Ctx = createContext<ToolContextValue>({
  activeTool: null,
  setActiveTool: () => {},
});

export function ToolProvider({ children }: { children: ReactNode }) {
  const [activeTool, setRaw] = useState<string | null>(() => localStorage.getItem(STORAGE_KEY));

  const setActiveTool = useCallback((name: string | null) => {
    setRaw(name);
    if (name) {
      localStorage.setItem(STORAGE_KEY, name);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) setRaw(e.newValue);
    };
    window.addEventListener('storage', handler);
    return () => window.removeEventListener('storage', handler);
  }, []);

  return <Ctx.Provider value={{ activeTool, setActiveTool }}>{children}</Ctx.Provider>;
}

export function useActiveTool() {
  return useContext(Ctx);
}
