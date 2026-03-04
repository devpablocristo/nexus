import { createContext, useCallback, useContext, useState } from 'react';

const STORAGE_KEY = 'nexus_api_key';

type AuthContextValue = {
  apiKey: string | null;
  login: (key: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue>({
  apiKey: null,
  login: () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [apiKey, setApiKey] = useState<string | null>(
    () => localStorage.getItem(STORAGE_KEY),
  );

  const login = useCallback((key: string) => {
    localStorage.setItem(STORAGE_KEY, key);
    setApiKey(key);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    setApiKey(null);
  }, []);

  return (
    <AuthContext.Provider value={{ apiKey, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
