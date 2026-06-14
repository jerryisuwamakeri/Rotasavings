"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api, currentUser, money, type MyGroup, type Reputation, type Transaction, type User } from "@/lib/api";
import { BarChart, ChartCard, Donut, Ring } from "../components/charts";

const TX_LABEL: Record<string, string> = {
  ContributionMade: "Contribution",
  ContributionMissed: "Missed",
  PayoutReceived: "Payout",
  GroupExit: "Left group",
  GroupExpulsion: "Removed",
};

// bucket contribution amounts by short month label
function monthlyContributions(txns: Transaction[]) {
  const buckets = new Map<string, number>();
  const now = new Date();
  for (let i = 5; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    buckets.set(d.toLocaleString(undefined, { month: "short" }), 0);
  }
  for (const t of txns) {
    if (t.type !== "ContributionMade") continue;
    const d = new Date(t.timestamp);
    const key = d.toLocaleString(undefined, { month: "short" });
    if (buckets.has(key)) buckets.set(key, (buckets.get(key) || 0) + t.amount);
  }
  return [...buckets.entries()].map(([label, value]) => ({ label, value: value / 100 }));
}

export default function MemberDashboard() {
  const [me, setMe] = useState<User | null>(null);
  const [groups, setGroups] = useState<MyGroup[]>([]);
  const [rep, setRep] = useState<Reputation | null>(null);
  const [txns, setTxns] = useState<Transaction[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const u = currentUser();
    setMe(u);
    try {
      const [g, t] = await Promise.all([api.myGroups(), api.myTransactions()]);
      setGroups(g.groups || []);
      setTxns(t.transactions || []);
      if (u) setRep(await api.reputation(u.id));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const stateCounts = groups.reduce<Record<string, number>>((acc, g) => {
    acc[g.group.state] = (acc[g.group.state] || 0) + 1;
    return acc;
  }, {});
  const groupDonut = Object.entries(stateCounts).map(([label, value]) => ({ label, value }));

  return (
    <div className="stack">
      <div className="page-head between">
        <div>
          <h1>Overview</h1>
          <p>{me?.kyc_status === "approved" ? "Verified account." : "KYC pending — browsing is open; joining unlocks on approval."}</p>
        </div>
        <Link href="/dashboard/groups/new" className="btn btn-primary">Create group</Link>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="kpi-row">
        <div className="kpi"><div className="value">{groups.length}</div><div className="label">My groups</div></div>
        <div className="kpi"><div className="value">{rep ? `${Math.round(rep.reliability_ratio * 100)}%` : "—"}</div><div className="label">Reliability</div></div>
        <div className="kpi"><div className="value amount">{rep ? money(rep.total_contributed) : "—"}</div><div className="label">Contributed</div></div>
        <div className="kpi"><div className="value">{rep?.payouts_received ?? "—"}</div><div className="label">Payouts</div></div>
      </div>

      {/* charts */}
      <div className="grid-2">
        <ChartCard title="Contributions (last 6 months)" big={rep ? money(rep.total_contributed) : ""}>
          <BarChart data={monthlyContributions(txns)} format={(n) => "₦" + n.toLocaleString()} />
        </ChartCard>
        <ChartCard title="Reliability">
          <div className="ring-wrap" style={{ justifyContent: "space-between" }}>
            <Ring value={rep?.reliability_ratio ?? 1} label="on-time rate" />
            <div className="mono" style={{ fontSize: 13, color: "var(--muted)", lineHeight: 1.9 }}>
              <div>made&nbsp;&nbsp;<b style={{ color: "var(--fg)" }}>{rep?.contributions_made ?? 0}</b></div>
              <div>missed&nbsp;<b style={{ color: "var(--fg)" }}>{rep?.contributions_missed ?? 0}</b></div>
              <div>payouts&nbsp;<b style={{ color: "var(--fg)" }}>{rep?.payouts_received ?? 0}</b></div>
            </div>
          </div>
        </ChartCard>
      </div>

      <div className="grid-2">
        <div className="section-card">
          <div className="between" style={{ marginBottom: 10 }}>
            <h3 style={{ margin: 0 }}>My groups</h3>
            <Link href="/dashboard/browse" className="btn btn-text">Browse</Link>
          </div>
          {groups.length === 0 ? (
            <div className="empty">No circles yet. <Link href="/dashboard/browse"><b>Find one</b></Link> or <Link href="/dashboard/groups/new"><b>start one</b></Link>.</div>
          ) : (
            <div className="list">
              {groups.map(({ group, organizer }) => (
                <Link key={group.id} href={`/dashboard/groups/${group.id}`} className="list-row">
                  <div>
                    <div className="t">{group.name} {organizer && <span className="role-pill">organizer</span>}</div>
                    <div className="s">{money(group.contribution_amount)}/cycle · {group.members.length}/{group.max_members}</div>
                  </div>
                  <span className={`badge ${group.state === "ACTIVE" ? "ok" : ""}`}>{group.state}</span>
                </Link>
              ))}
            </div>
          )}
        </div>

        <ChartCard title="Groups by state">
          {groupDonut.length === 0 ? (
            <div className="empty" style={{ padding: "26px 0" }}>No groups yet.</div>
          ) : (
            <Donut data={groupDonut} centerValue={String(groups.length)} centerLabel="groups" />
          )}
        </ChartCard>
      </div>

      <div className="section-card">
        <div className="between" style={{ marginBottom: 10 }}>
          <h3 style={{ margin: 0 }}>Recent activity</h3>
          <Link href="/dashboard/transactions" className="btn btn-text">All</Link>
        </div>
        {txns.length === 0 ? (
          <div className="empty">No transactions yet.</div>
        ) : (
          <table>
            <tbody>
              {txns.slice(0, 6).map((t) => {
                const credit = t.type === "PayoutReceived";
                return (
                  <tr key={t.id}>
                    <td><span className={`badge ${credit ? "ok" : ""}`}>{TX_LABEL[t.type] || t.type}</span></td>
                    <td className="muted mono">{new Date(t.timestamp).toLocaleDateString()}</td>
                    <td style={{ textAlign: "right" }} className={`amount mono tx-amt ${credit ? "pos" : t.type === "ContributionMissed" ? "neg" : ""}`}>
                      {t.amount ? (credit ? "+" : "") + money(t.amount) : "—"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
