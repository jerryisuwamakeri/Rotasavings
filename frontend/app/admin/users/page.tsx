"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type Role, type User } from "@/lib/api";

export default function AdminUsers() {
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    try {
      setUsers((await api.adminUsers()).users || []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function act(id: string, fn: () => Promise<unknown>) {
    setBusy(id);
    setError("");
    try {
      await fn();
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
        <h1>Users</h1>
        <p>Full account control - roles, KYC, and suspension.</p>
      </div>
      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}

      <div className="section-card">
        <table>
          <thead>
            <tr><th>Name / Email</th><th>Role</th><th>KYC</th><th>Status</th><th style={{ textAlign: "right" }}>Actions</th></tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>
                  <div className="t" style={{ fontWeight: 600 }}>{u.display_name}</div>
                  <div className="s muted">{u.email}</div>
                </td>
                <td>
                  <select
                    value={u.role}
                    disabled={busy === u.id}
                    onChange={(e) => act(u.id, () => api.setRole(u.id, e.target.value as Role))}
                    style={{ width: 110 }}
                  >
                    <option value="member">member</option>
                    <option value="admin">admin</option>
                  </select>
                </td>
                <td>
                  {u.kyc_status === "pending" ? (
                    <span className="inline-list">
                      <button className="btn btn-primary" disabled={busy === u.id} onClick={() => act(u.id, () => api.decideKYC(u.id, true))}>Approve</button>
                      <button className="btn danger" disabled={busy === u.id} onClick={() => act(u.id, () => api.decideKYC(u.id, false))}>Reject</button>
                    </span>
                  ) : (
                    <span className={`badge ${u.kyc_status === "approved" ? "ok" : "crit"}`}>{u.kyc_status}</span>
                  )}
                </td>
                <td><span className={`badge ${u.status === "active" ? "ok" : "crit"}`}>{u.status}</span></td>
                <td style={{ textAlign: "right" }}>
                  {u.role !== "admin" && (
                    <button className={`btn ${u.status === "active" ? "danger" : "btn-ghost"}`} disabled={busy === u.id}
                      onClick={() => act(u.id, () => (u.status === "active" ? api.suspend(u.id) : api.activateUser(u.id)))}>
                      {u.status === "active" ? "Suspend" : "Activate"}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
