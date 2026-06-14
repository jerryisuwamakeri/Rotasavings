"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type User } from "@/lib/api";

export default function AdminKYC() {
  const [pending, setPending] = useState<User[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    try {
      setPending((await api.pendingKYC()).pending || []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function decide(id: string, approve: boolean) {
    setBusy(id);
    try {
      await api.decideKYC(id, approve);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>KYC review</h1>
        <p>Approve identity verification to unlock joining and creating groups.</p>
      </div>
      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}

      <div className="section-card">
        {pending.length === 0 ? (
          <div className="empty" style={{ padding: "30px 0" }}>No pending reviews. You&apos;re all caught up.</div>
        ) : (
          <table>
            <thead><tr><th>Name</th><th>Email</th><th>Wallet</th><th style={{ textAlign: "right" }}>Decision</th></tr></thead>
            <tbody>
              {pending.map((u) => (
                <tr key={u.id}>
                  <td style={{ fontWeight: 600 }}>{u.display_name}</td>
                  <td className="muted">{u.email}</td>
                  <td className="muted mono">{u.wallet_address}</td>
                  <td style={{ textAlign: "right" }}>
                    <span className="inline-list" style={{ justifyContent: "flex-end" }}>
                      <button className="btn btn-primary" disabled={busy === u.id} onClick={() => decide(u.id, true)}>Approve</button>
                      <button className="btn danger" disabled={busy === u.id} onClick={() => decide(u.id, false)}>Reject</button>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
