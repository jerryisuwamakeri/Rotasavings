"use client";

import { useCallback, useEffect, useState } from "react";
import { api, currentUser, money, type Group } from "@/lib/api";

export default function BrowsePage() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState("");
  const me = currentUser();

  const load = useCallback(async () => {
    try {
      const g = await api.groups();
      setGroups(g.groups || []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function join(id: string) {
    setBusy(id);
    setError("");
    setMsg("");
    try {
      await api.requestJoin(id);
      setMsg("Request sent. The organizer will review it.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not request to join");
    } finally {
      setBusy("");
    }
  }

  const open = groups.filter((g) => g.state === "CREATED");

  return (
    <>
      <div className="page-head">
        <h1>Browse groups</h1>
        <p>Open circles accepting members. Request to join; the organizer approves.</p>
      </div>

      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}
      {msg && <div className="ok-msg" style={{ marginBottom: 14 }}>{msg}</div>}

      {open.length === 0 ? (
        <div className="section-card"><div className="empty">No open groups right now. Why not <a className="clay" href="/dashboard/groups/new">start one</a>?</div></div>
      ) : (
        <div className="grid-cards">
          {open.map((g) => {
            const mine = me && g.members.includes(me.id);
            const full = g.members.length >= g.max_members;
            return (
              <div key={g.id} className="card-link" style={{ cursor: "default" }}>
                <div className="t" style={{ fontSize: 17, fontWeight: 600 }}>{g.name}</div>
                <div className="s muted" style={{ margin: "6px 0 14px" }}>
                  {money(g.contribution_amount)} / cycle &middot; {g.members.length}/{g.max_members} members
                </div>
                {mine ? (
                  <span className="badge ok">You&apos;re in this group</span>
                ) : full ? (
                  <span className="badge warn">Full</span>
                ) : (
                  <button className="btn btn-primary" disabled={busy === g.id} onClick={() => join(g.id)}>
                    {busy === g.id ? "Requesting..." : "Request to join"}
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </>
  );
}
