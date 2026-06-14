"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { login, isAuthed, currentUser, logout } from "@/lib/api";
import SiteHeader from "../components/SiteHeader";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Only auto-redirect when we are genuinely signed in (token AND a known
    // user). A dangling token with no user is cleared so the form is usable -
    // this prevents an infinite redirect loop with the dashboard shell.
    if (!isAuthed()) return;
    const u = currentUser();
    if (u) {
      router.replace(u.role === "admin" ? "/admin" : "/dashboard");
    } else {
      logout();
    }
  }, [router]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const user = await login(email, password);
      router.replace(user.role === "admin" ? "/admin" : "/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <SiteHeader />
      <div className="auth-wrap">
        <div className="auth-card">
          <div className="card">
            <h1 style={{ margin: 0 }}>Welcome back</h1>
            <p className="muted" style={{ marginTop: 6 }}>Sign in to your circles.</p>
            <form onSubmit={onSubmit}>
              <div className="field">
                <label htmlFor="email">Email</label>
                <input id="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" type="email" autoComplete="email" />
              </div>
              <div className="field">
                <label htmlFor="password">Password</label>
                <input id="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Your password" type="password" autoComplete="current-password" />
              </div>
              {error && <div className="error" style={{ marginTop: 14 }}>{error}</div>}
              <button className="btn btn-primary btn-block btn-lg" disabled={busy} type="submit" style={{ marginTop: 20 }}>
                {busy ? "Signing in..." : "Sign in"}
              </button>
            </form>
          </div>
          <p className="muted" style={{ textAlign: "center", marginTop: 18, fontSize: 14 }}>
            New here? <Link href="/register" className="clay">Create an account</Link> &middot; <Link href="/">Home</Link>
          </p>
        </div>
      </div>
    </>
  );
}
