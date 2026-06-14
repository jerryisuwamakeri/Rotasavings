"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api, money, type Liquidity, type Overview, type Transaction } from "@/lib/api";
import { BarChart, ChartCard, Donut } from "../components/charts";

function dailyVolume(txns: Transaction[]) {
  const buckets = new Map<string, number>();
  const now = new Date();
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i);
    buckets.set(d.toLocaleDateString(undefined, { weekday: "short" }), 0);
  }
  for (const t of txns) {
    const d = new Date(t.timestamp);
    const key = d.toLocaleDateString(undefined, { weekday: "short" });
    if (buckets.has(key)) buckets.set(key, (buckets.get(key) || 0) + (t.amount || 0));
  }
  return [...buckets.entries()].map(([label, value]) => ({ label, value: value / 100 }));
}

export default function AdminOverview() {
  const [overview, setOverview] = useState<Overview | null>(null);
  const [liquidity, setLiquidity] = useState<Liquidity[]>([]);
  const [txns, setTxns] = useState<Transaction[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [ov, liq, tx] = await Promise.all([api.overview(), api.liquidity(), api.adminTransactions()]);
      setOverview(ov);
      setLiquidity(liq.groups || []);
      setTxns(tx.transactions || []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const groupsDonut = overview ? Object.entries(overview.groups_by_state).map(([label, value]) => ({ label, value })) : [];
  const typeCounts = txns.reduce<Record<string, number>>((acc, t) => {
    const k = t.type === "PayoutReceived" ? "Payouts" : t.type === "ContributionMissed" ? "Defaults" : "Contributions";
    acc[k] = (acc[k] || 0) + 1;
    return acc;
  }, {});
  const typeBars = ["Contributions", "Payouts", "Defaults"].map((label) => ({ label, value: typeCounts[label] || 0 }));

  return (
    <div className="stack">
      <div className="page-head">
        <h1>Overview</h1>
        <p>Platform analytics. Manage users, groups, transactions and webhooks from the sidebar.</p>
      </div>

      {error && <div className="error">{error}</div>}

      <div className="kpi-row">
        <Kpi label="Users" value={overview?.users} href="/admin/users" />
        <Kpi label="Groups" value={overview?.total_groups} href="/admin/groups" />
        <Kpi label="Pending KYC" value={overview?.pending_kyc} href="/admin/kyc" />
        <Kpi label="Escrow" value={overview ? money(overview.total_escrow) : undefined} />
        <Kpi label="Payouts" value={overview?.total_payouts} href="/admin/transactions" />
      </div>

      <div className="grid-2">
        <ChartCard title="Volume (last 7 days)" big={money(txns.reduce((s, t) => s + (t.amount || 0), 0))}>
          <BarChart data={dailyVolume(txns)} format={(n) => "₦" + n.toLocaleString()} />
        </ChartCard>
        <ChartCard title="Activity breakdown" big={String(txns.length)}>
          <BarChart data={typeBars} />
        </ChartCard>
      </div>

      <div className="grid-2">
        <ChartCard title="Groups by state">
          {groupsDonut.length === 0 ? (
            <div className="empty" style={{ padding: "26px 0" }}>No groups.</div>
          ) : (
            <Donut data={groupsDonut} centerValue={String(overview?.total_groups ?? 0)} centerLabel="groups" />
          )}
        </ChartCard>

        <div className="section-card">
          <div className="between" style={{ marginBottom: 10 }}>
            <h3 style={{ margin: 0 }}>Liquidity stress</h3>
            <Link href="/admin/groups" className="btn btn-text">Groups</Link>
          </div>
          {liquidity.length === 0 ? (
            <div className="empty" style={{ padding: "20px 0" }}>No active groups.</div>
          ) : (
            <table>
              <thead><tr><th>Group</th><th>Escrow</th><th>Collapse</th><th>State</th></tr></thead>
              <tbody>
                {liquidity.map((l) => (
                  <tr key={l.group_id}>
                    <td className="muted mono">{l.group_id.slice(0, 8)}…</td>
                    <td className="mono">{money(l.escrow_balance)}</td>
                    <td className="mono">{(l.collapse_probability * 100).toFixed(0)}%</td>
                    <td><span className={`badge ${l.level === "ok" ? "ok" : ""}`}>{l.level}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}

function Kpi({ label, value, href }: { label: string; value?: number | string; href?: string }) {
  const inner = (
    <div className="kpi" style={href ? { cursor: "pointer" } : undefined}>
      <div className="value">{value ?? "—"}</div>
      <div className="label">{label}</div>
    </div>
  );
  return href ? <Link href={href}>{inner}</Link> : inner;
}
