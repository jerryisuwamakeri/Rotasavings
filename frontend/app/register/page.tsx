"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { register, login } from "@/lib/api";
import SiteHeader from "../components/SiteHeader";

export default function RegisterPage() {
  const router = useRouter();
  const [form, setForm] = useState({ display_name: "", email: "", wallet_address: "", password: "" });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  function set(k: keyof typeof form) {
    return (e: React.ChangeEvent<HTMLInputElement>) => setForm({ ...form, [k]: e.target.value });
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await register(form);
      // Auto sign-in after registering.
      const user = await login(form.email, form.password);
      router.replace(user.role === "admin" ? "/admin" : "/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create account");
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
            <h1 style={{ margin: 0 }}>Create your account</h1>
            <p className="muted" style={{ marginTop: 6 }}>Join Rotasavings and start a circle. KYC review unlocks joining groups.</p>
            <form onSubmit={onSubmit}>
              <div className="field">
                <label>Full name</label>
                <input value={form.display_name} onChange={set("display_name")} placeholder="Amara Okafor" />
              </div>
              <div className="field">
                <label>Email</label>
                <input type="email" value={form.email} onChange={set("email")} placeholder="you@example.com" autoComplete="email" />
              </div>
              <div className="field">
                <label>Wallet address</label>
                <input value={form.wallet_address} onChange={set("wallet_address")} placeholder="0x..." />
              </div>
              <div className="field">
                <label>Password</label>
                <input type="password" value={form.password} onChange={set("password")} placeholder="At least 8 characters" autoComplete="new-password" />
              </div>
              {error && <div className="error" style={{ marginTop: 14 }}>{error}</div>}
              <button className="btn btn-primary btn-block btn-lg" disabled={busy} type="submit" style={{ marginTop: 20 }}>
                {busy ? "Creating..." : "Create account"}
              </button>
            </form>
          </div>
          <p className="muted" style={{ textAlign: "center", marginTop: 18, fontSize: 14 }}>
            Already have an account? <Link href="/login" className="clay">Sign in</Link>
          </p>
        </div>
      </div>
    </>
  );
}
