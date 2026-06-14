"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";

export default function NewGroupPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [amount, setAmount] = useState("50000");
  const [cycle, setCycle] = useState("168h");
  const [maxMembers, setMaxMembers] = useState("6");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const g = await api.createGroup({
        name: name.trim(),
        contribution_amount: Math.round(Number(amount) * 100), // naira -> kobo
        cycle_length: cycle,
        max_members: Number(maxMembers),
      });
      router.replace(`/dashboard/groups/${g.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create group");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>Start a savings circle</h1>
        <p>You become the organizer. Invite members or approve requests, then activate to begin the rotation.</p>
      </div>

      <div className="section-card" style={{ maxWidth: 560 }}>
        <form className="form" onSubmit={onSubmit}>
          <div className="field">
            <label>Group name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Lagos Traders Circle" />
          </div>
          <div className="field">
            <label>Contribution per cycle (₦)</label>
            <input type="number" min="1" value={amount} onChange={(e) => setAmount(e.target.value)} />
            <span className="hint">Each member pays this every cycle.</span>
          </div>
          <div className="field">
            <label>Cycle length</label>
            <select value={cycle} onChange={(e) => setCycle(e.target.value)}>
              <option value="24h">Daily</option>
              <option value="168h">Weekly</option>
              <option value="336h">Every 2 weeks</option>
              <option value="720h">Monthly</option>
            </select>
          </div>
          <div className="field">
            <label>Maximum members</label>
            <input type="number" min="2" max="50" value={maxMembers} onChange={(e) => setMaxMembers(e.target.value)} />
            <span className="hint">The rotation runs for one cycle per member.</span>
          </div>
          {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
          <div className="form-actions">
            <button className="btn btn-primary" type="submit" disabled={busy}>{busy ? "Creating..." : "Create group"}</button>
            <Link href="/dashboard" className="btn btn-ghost">Cancel</Link>
          </div>
        </form>
      </div>
    </>
  );
}
