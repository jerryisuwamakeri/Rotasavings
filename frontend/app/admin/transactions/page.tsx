"use client";

import { useEffect, useState } from "react";
import { api, money, type Transaction } from "@/lib/api";

const LABEL: Record<string, string> = {
  ContributionMade: "Contribution",
  ContributionMissed: "Default",
  PayoutReceived: "Payout",
  GroupExit: "Exit",
  GroupExpulsion: "Expulsion",
};

export default function AdminTransactions() {
  const [txns, setTxns] = useState<Transaction[]>([]);
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    api
      .adminTransactions()
      .then((t) => setTxns(t.transactions || []))
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setLoaded(true));
  }, []);

  return (
    <>
      <div className="page-head">
        <h1>Transactions</h1>
        <p>Platform-wide financial ledger - every contribution, payout and default, newest first.</p>
      </div>
      {error && <div className="error">{error}</div>}

      <div className="section-card">
        {loaded && txns.length === 0 ? (
          <div className="empty">No transactions yet.</div>
        ) : (
          <table>
            <thead>
              <tr><th>Type</th><th>User</th><th>Group</th><th>Cycle</th><th>When</th><th style={{ textAlign: "right" }}>Amount</th></tr>
            </thead>
            <tbody>
              {txns.map((t) => {
                const credit = t.type === "PayoutReceived";
                const missed = t.type === "ContributionMissed";
                return (
                  <tr key={t.id}>
                    <td><span className={`badge ${credit ? "ok" : missed ? "crit" : ""}`}>{LABEL[t.type] || t.type}</span></td>
                    <td className="muted mono">{t.user_id.slice(0, 8)}…</td>
                    <td className="muted mono">{t.group_id.slice(0, 8)}…</td>
                    <td>{t.cycle_index}</td>
                    <td className="muted">{new Date(t.timestamp).toLocaleString()}</td>
                    <td style={{ textAlign: "right" }} className={`amount tx-amt ${credit ? "pos" : missed ? "neg" : ""}`}>
                      {t.amount ? (credit ? "+" : missed ? "−" : "") + money(t.amount) : "—"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
