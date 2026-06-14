"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  api,
  currentUser,
  money,
  type Cycle,
  type CycleStatus,
  type Group,
  type JoinRequest,
  type Membership,
} from "@/lib/api";

export default function GroupDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const router = useRouter();
  const me = currentUser();

  const [group, setGroup] = useState<Group | null>(null);
  const [members, setMembers] = useState<Membership[]>([]);
  const [cycles, setCycles] = useState<Cycle[]>([]);
  const [requests, setRequests] = useState<JoinRequest[]>([]);
  const [status, setStatus] = useState<CycleStatus | null>(null);
  const [inviteId, setInviteId] = useState("");
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);

  const isOrganizer = !!group && !!me && group.organizer_id === me.id;
  const current = cycles.find((c) => !c.settled);

  const load = useCallback(async () => {
    try {
      const g = await api.group(id);
      setGroup(g);
      const [m, c] = await Promise.all([api.members(id), api.cycles(id)]);
      setMembers(m.members || []);
      setCycles(c.cycles || []);
      const cur = (c.cycles || []).find((x) => !x.settled);
      if (g.state === "ACTIVE" && cur) setStatus(await api.cycleStatus(id, cur.index));
      else setStatus(null);
      if (me && g.organizer_id === me.id && g.state === "CREATED") {
        setRequests((await api.listJoinRequests(id)).join_requests || []);
      }
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  async function run(fn: () => Promise<unknown>, okMsg?: string) {
    setBusy(true);
    setError("");
    setMsg("");
    try {
      await fn();
      if (okMsg) setMsg(okMsg);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    } finally {
      setBusy(false);
    }
  }

  if (!group) {
    return <div className="page-head"><h1>Group</h1>{error ? <div className="error">{error}</div> : <p>Loading…</p>}</div>;
  }

  const iPaid = status?.members.find((m) => m.user_id === me?.id)?.paid;
  const label = (uid: string) => (uid === me?.id ? "You" : uid.slice(0, 8) + "…");

  return (
    <>
      <div className="page-head between">
        <div>
          <Link href="/dashboard" className="btn btn-text" style={{ paddingLeft: 0 }}>← Back</Link>
          <h1 style={{ marginTop: 6 }}>{group.name}</h1>
          <p>{money(group.contribution_amount)} / cycle &middot; {group.members.length}/{group.max_members} members &middot; <span className={`badge ${group.state === "ACTIVE" ? "ok" : group.state === "CLOSED" ? "" : "warn"}`}>{group.state}</span></p>
        </div>
        {isOrganizer && group.state === "CREATED" && (
          <button className="btn btn-primary" disabled={busy || group.members.length < 2} onClick={() => run(() => api.activate(id), "Group activated - the rotation has begun.")}>
            Activate rotation
          </button>
        )}
      </div>

      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}
      {msg && <div className="ok-msg" style={{ marginBottom: 14 }}>{msg}</div>}

      {/* Current cycle (active) */}
      {group.state === "ACTIVE" && status && (
        <div className="section-card">
          <div className="between">
            <h3 style={{ margin: 0 }}>Cycle {status.cycle.index} &middot; pays {label(status.cycle.payout_user)}</h3>
            <span className="muted">{money(status.collected)} / {money(status.expected)} collected</span>
          </div>
          <table style={{ marginTop: 12 }}>
            <tbody>
              {status.members.map((m) => (
                <tr key={m.user_id}>
                  <td>{label(m.user_id)}</td>
                  <td style={{ textAlign: "right" }}>
                    <span className={`badge ${m.paid ? "ok" : "warn"}`}>{m.paid ? "paid" : "pending"}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="form-actions">
            {!iPaid && (
              <button className="btn btn-primary" disabled={busy} onClick={() => run(() => api.contribute(id, status.cycle.index, "momo:self"), "Contribution recorded.")}>
                Contribute {money(group.contribution_amount)}
              </button>
            )}
            {isOrganizer && (
              <button className="btn btn-ghost" disabled={busy} onClick={() => run(() => api.settle(id, status.cycle.index), "Cycle settled.")}>
                Settle cycle
              </button>
            )}
          </div>
        </div>
      )}

      {/* Members */}
      <div className="section-card">
        <h3>Members</h3>
        <div className="list">
          {members.map((m) => (
            <div key={m.id} className="list-row">
              <div>
                <span className="t">{label(m.user_id)}</span>{" "}
                {m.organizer && <span className="role-pill">organizer</span>}
              </div>
              {isOrganizer && group.state === "CREATED" && !m.organizer && (
                <button className="btn danger" disabled={busy} onClick={() => run(() => api.removeMember(id, m.user_id), "Member removed.")}>Remove</button>
              )}
            </div>
          ))}
        </div>
        {!isOrganizer && group.state === "CREATED" && (
          <div className="form-actions">
            <button className="btn btn-ghost" disabled={busy} onClick={() => run(() => api.leave(id).then(() => router.replace("/dashboard")))}>Leave group</button>
          </div>
        )}
      </div>

      {/* Organizer: join requests + invite (only while forming) */}
      {isOrganizer && group.state === "CREATED" && (
        <div className="section-card">
          <h3>Join requests</h3>
          {requests.filter((r) => r.status === "pending").length === 0 ? (
            <div className="empty" style={{ padding: "20px 0" }}>No pending requests.</div>
          ) : (
            <div className="list">
              {requests.filter((r) => r.status === "pending").map((r) => (
                <div key={r.id} className="list-row">
                  <span className="mono">{r.user_id.slice(0, 10)}…</span>
                  <div className="inline-list">
                    <button className="btn btn-primary" disabled={busy} onClick={() => run(() => api.decideJoin(r.id, true), "Approved.")}>Approve</button>
                    <button className="btn danger" disabled={busy} onClick={() => run(() => api.decideJoin(r.id, false), "Rejected.")}>Reject</button>
                  </div>
                </div>
              ))}
            </div>
          )}
          <div className="form-actions" style={{ alignItems: "center" }}>
            <input value={inviteId} onChange={(e) => setInviteId(e.target.value)} placeholder="Invite by user ID" style={{ maxWidth: 320 }} />
            <button className="btn btn-ghost" disabled={busy || !inviteId.trim()} onClick={() => run(() => api.invite(id, inviteId.trim()).then(() => setInviteId("")), "Invitation sent.")}>Invite</button>
          </div>
        </div>
      )}

      {/* Cycle schedule */}
      {cycles.length > 0 && (
        <div className="section-card">
          <h3>Schedule</h3>
          <table>
            <thead><tr><th>Cycle</th><th>Payee</th><th>Deadline</th><th style={{ textAlign: "right" }}>Status</th></tr></thead>
            <tbody>
              {cycles.map((c) => (
                <tr key={c.index}>
                  <td>{c.index}</td>
                  <td>{label(c.payout_user)}</td>
                  <td className="muted">{new Date(c.deadline).toLocaleDateString()}</td>
                  <td style={{ textAlign: "right" }}><span className={`badge ${c.settled ? "ok" : ""}`}>{c.settled ? "settled" : "open"}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
