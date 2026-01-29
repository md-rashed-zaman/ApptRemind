"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { useAuthStore } from "../../../lib/auth-store";

export default function LogoutPage() {
  const router = useRouter();
  const { logout } = useAuthStore();

  useEffect(() => {
    logout();
    router.replace("/auth/login");
  }, [logout, router]);

  return null;
}
