import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../hooks/useAuth";

export function AuthCallback() {
  const navigate = useNavigate();
  const { completeAuth } = useAuth();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let redirectTimer: ReturnType<typeof setTimeout>;
    let cancelled = false;

    const handleCallback = async () => {
      const urlParams = new URLSearchParams(window.location.search);
      const code = urlParams.get("code");

      if (!code) {
        if (!cancelled) setError("No authorization code found");
        return;
      }

      try {
        const state = urlParams.get("state") || undefined;
        const response = await api.githubCallback({ code, state });
        completeAuth(response);
        if (!cancelled) navigate("/");
      } catch (err) {
        console.error("Auth callback failed:", err);
        if (!cancelled) setError("Authentication failed. Please try again.");
        redirectTimer = setTimeout(() => {
          if (!cancelled) navigate("/");
        }, 3000);
      }
    };

    handleCallback();
    return () => {
      cancelled = true;
      clearTimeout(redirectTimer);
    };
  }, [completeAuth, navigate]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-900">
        <div className="bg-slate-800 rounded-lg p-8 text-center max-w-md">
          <svg
            className="mx-auto h-12 w-12 text-red-500 mb-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <h2 className="text-xl font-semibold text-white mb-2">Authentication Error</h2>
          <p className="text-slate-400">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-900">
      <div className="bg-slate-800 rounded-lg p-8 text-center">
        <div className="spinner mx-auto mb-4" />
        <p className="text-slate-400">Completing authentication...</p>
      </div>
    </div>
  );
}
