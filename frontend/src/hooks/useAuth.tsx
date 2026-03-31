import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import type { GitHubAuthResponse, User } from '../types';
import { api } from '../api/client';

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  login: () => Promise<void>;
  logout: () => void;
  linkWallet: (walletAddress: string) => Promise<User>;
  refreshUser: () => Promise<void>;
  completeAuth: (session: GitHubAuthResponse) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    checkAuth();

    const handleExpired = () => {
      setUser(null);
      localStorage.removeItem('token');
    };
    window.addEventListener('auth-expired', handleExpired);
    return () => window.removeEventListener('auth-expired', handleExpired);
  }, []);

  const checkAuth = async () => {
    const token = localStorage.getItem('token');
    if (!token) {
      setIsLoading(false);
      return;
    }

    try {
      const userData = await api.getMe();
      setUser(userData);
    } catch (error) {
      console.error('Auth check failed:', error);
      localStorage.removeItem('token');
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  };

  const login = useCallback(async () => {
    try {
      const { auth_url } = await api.getGithubAuthUrl();
      window.location.href = auth_url;
    } catch (error) {
      console.error('Login failed:', error);
      throw error;
    }
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('token');
    setUser(null);
  }, []);

  const linkWallet = useCallback(async (walletAddress: string): Promise<User> => {
    const updatedUser = await api.linkWallet({ wallet_address: walletAddress });
    setUser(updatedUser);
    return updatedUser;
  }, []);

  const refreshUser = useCallback(async () => {
    const userData = await api.getMe();
    setUser(userData);
  }, []);

  const completeAuth = useCallback((session: GitHubAuthResponse) => {
    localStorage.setItem('token', session.token);
    setUser(session.user);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, isLoading, login, logout, linkWallet, refreshUser, completeAuth }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
