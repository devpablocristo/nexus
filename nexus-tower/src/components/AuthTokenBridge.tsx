import { useEffect } from 'react';
import { useAuth } from '@clerk/clerk-react';

import { setTokenGetter } from '../api/client';

export function AuthTokenBridge() {
  const { getToken } = useAuth();

  useEffect(() => {
    setTokenGetter(() => getToken());
    return () => {
      setTokenGetter(null);
    };
  }, [getToken]);

  return null;
}

