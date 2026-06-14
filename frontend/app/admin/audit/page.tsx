"use client";

import { useEffect, useState } from "react";
import { api, type AuditEntry } from "@/lib/api";

export default function AdminAudit() {
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .audit()
      .then((a) => setAudit(a.audit || []))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"));
  }, []);

  return (
    <>
      <div className="page-head">
        <h1>Audit log</h1>
        <p>Every privileged action, newest first. Append-only.</p>
      </div>
      {error && <div className="error">{error}</div>}

      <div className="section-card">
        {audit.length === 0 ? (
          <div className="empty" style={{ padding: "20px 0" }}>No admin actions recorded yet.</div>
        ) : (
          <table>
            <thead><tr><th>When</th><th>Action</th><th>Actor</th><th>Target</th></tr></thead>
            <tbody>
              {audit.map((a) => (
                <tr key={a.id}>
                  <td className="muted">{new Date(a.created_at).toLocaleString()}</td>
                  <td><span className="badge">{a.action}</span></td>
                  <td className="muted mono">{a.actor_id.slice(0, 8)}…</td>
                  <td className="muted mono">{a.target.slice(0, 10)}…</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
