import { create } from "zustand";

import { apiLogin, apiMe, apiRegister, tokenStore } from "./api";

type User = {
  id: string;
  business_id: string;
  email: string;
  role: string;
};

type AuthState = {
  user: User | null;
  loading: boolean;
  hydrated: boolean;
  error: string | null;
  hydrate: () => Promise<void>;
  login: (email: string, password: string) => Promise<boolean>;
  register: (payload: {
    email: string;
    password: string;
    business_name: string;
  }) => Promise<boolean>;
  logout: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: false,
  hydrated: false,
  error: null,
  hydrate: async () => {
    const tokens = tokenStore.get();
    if (!tokens?.accessToken || !tokens.refreshToken) {
      set({ user: null, loading: false, hydrated: true, error: null });
      return;
    }
    set({ loading: true, error: null });
    try {
      const me = await apiMe();
      set({ user: me, loading: false, hydrated: true });
    } catch (err) {
      set({
        user: null,
        loading: false,
        hydrated: true,
        error: "Failed to load session"
      });
    }
  },
  login: async (email: string, password: string) => {
    set({ loading: true, error: null });
    try {
      await apiLogin(email, password);
      const me = await apiMe();
      set({ user: me, loading: false, hydrated: true });
      return true;
    } catch {
      set({ loading: false, error: "Invalid email or password" });
      return false;
    }
  },
  register: async (payload) => {
    set({ loading: true, error: null });
    try {
      await apiRegister(payload);
      const me = await apiMe();
      set({ user: me, loading: false, hydrated: true });
      return true;
    } catch {
      set({ loading: false, error: "Registration failed" });
      return false;
    }
  },
  logout: () => {
    tokenStore.set(null);
    set({ user: null, error: null, loading: false, hydrated: true });
  }
}));
