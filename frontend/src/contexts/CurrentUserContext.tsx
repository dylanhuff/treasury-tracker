/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, useEffect, useMemo, useCallback } from 'react';
import type { ReactNode } from 'react';
import type { User } from '../types/user';
import { fetchUsers } from '../services/api';

const LOCAL_STORAGE_KEY = 'currentUserId';

interface CurrentUserContextType {
  users: User[];
  currentUser: User | null;
  setCurrentUserById: (userId: number) => void;
  updateUser: (user: User) => void;
  refreshUser: () => Promise<void>;
  isLoading: boolean;
  error: string | null;
}

const CurrentUserContext = createContext<CurrentUserContextType | undefined>(undefined);

interface CurrentUserProviderProps {
  children: ReactNode;
}

// Fetches users on mount, persists selection to localStorage, and restores on refresh.
export function CurrentUserProvider({ children }: CurrentUserProviderProps) {
  const [users, setUsers] = useState<User[]>([]);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    const signal = controller.signal;

    async function loadUsers() {
      try {
        setIsLoading(true);
        setError(null);

        const fetchedUsers = await fetchUsers();

        if (signal.aborted) return;

        setUsers(fetchedUsers);

        if (fetchedUsers.length === 0) {
          setCurrentUser(null);
          setIsLoading(false);
          return;
        }

        // Restore previously selected user from localStorage
        const storedUserId = localStorage.getItem(LOCAL_STORAGE_KEY);

        if (storedUserId) {
          const parsedId = parseInt(storedUserId, 10);
          const storedUser = fetchedUsers.find(user => user.id === parsedId);

          if (storedUser) {
            setCurrentUser(storedUser);
            setIsLoading(false);
            return;
          }
        }

        // Default to first user
        const firstUser = fetchedUsers[0];
        setCurrentUser(firstUser);
        localStorage.setItem(LOCAL_STORAGE_KEY, firstUser.id.toString());

        setIsLoading(false);
      } catch (err) {
        if (signal.aborted) return;

        const errorMessage = err instanceof Error ? err.message : 'Failed to load users';
        setError(errorMessage);
        setIsLoading(false);
      }
    }

    loadUsers();

    return () => {
      controller.abort();
    };
  }, []);

  const setCurrentUserById = useCallback((userId: number) => {
    const user = users.find(u => u.id === userId);
    if (user) {
      setCurrentUser(user);
      localStorage.setItem(LOCAL_STORAGE_KEY, userId.toString());
    }
  }, [users]);

  const updateUser = useCallback((user: User) => {
    setCurrentUser(user);
    setUsers(prevUsers => prevUsers.map(u => u.id === user.id ? user : u));
  }, []);

  const refreshUser = useCallback(async () => {
    if (!currentUser) return;

    try {
      const fetchedUsers = await fetchUsers();
      setUsers(fetchedUsers);

      const updated = fetchedUsers.find(u => u.id === currentUser.id);
      if (updated) {
        setCurrentUser(updated);
      }
    } catch {
      // Silently handle refresh errors
    }
  }, [currentUser]);

  const value = useMemo(() => ({
    users,
    currentUser,
    setCurrentUserById,
    updateUser,
    refreshUser,
    isLoading,
    error,
  }), [users, currentUser, setCurrentUserById, updateUser, refreshUser, isLoading, error]);

  return (
    <CurrentUserContext.Provider value={value}>
      {children}
    </CurrentUserContext.Provider>
  );
}

export function useCurrentUser(): CurrentUserContextType {
  const context = useContext(CurrentUserContext);

  if (context === undefined) {
    throw new Error('useCurrentUser must be used within a CurrentUserProvider');
  }

  return context;
}
