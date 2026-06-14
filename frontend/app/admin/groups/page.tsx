"use client";

import { useCallback, useEffect, useState } from "react";
import { api, money, type Group } from "@/lib/api";

export default function AdminGroups() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    try {
      setGroups((await api.adminGroups()).groups || []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function forceSettle(g: Group) {
    setBusy(g.id);
    setError("");
    try {
      // settle the first open cycle (current). The backend advances lifecycle.
      const current = g.total_cycles ? 0 : 0;
      await api.forceSettle(g.id, current);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not settle");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>Groups</h1>
        <p>Every circle on the platform. Intervene with an operator force-settle when needed.</p>
      </div>
      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}

      <div className="section-card">
        {groups.length === 0 ? (
          <div className="empty" style={{ padding: "20px 0" }}>No groups yet.</div>
        ) : (
          <table>
            <thead>
              <tr><th>Name</th><th>Contribution</th><th>Members</th><th>Cycles</th><th>State</th><th style={{ textAlign: "right" }}>Operator</th></tr>
            </thead>
            <tbody>
              {groups.map((g) => (
                <tr key={g.id}>
                  <td>
                    <div className="t" style={{ fontWeight: 600 }}>{g.name}</div>
                    <div className="s muted mono">{g.id.slice(0, 10)}…</div>
                  </td>
                  <td>{money(g.contribution_amount)}</td>
                  <td>{g.members.length}/{g.max_members}</td>
                  <td>{g.total_cycles || "—"}</td>
                  <td><span className={`badge ${g.state === "ACTIVE" ? "ok" : g.state === "CLOSED" ? "" : "warn"}`}>{g.state}</span></td>
                  <td style={{ textAlign: "right" }}>
                    {g.state === "ACTIVE" && (
                      <button className="btn btn-ghost" disabled={busy === g.id} onClick={() => forceSettle(g)}>Force-settle cycle 0</button>
                    )}
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
