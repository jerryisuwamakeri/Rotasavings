"use client";

import { useCallback, useEffect, useState } from "react";
import { api, type Webhook } from "@/lib/api";

export default function AdminWebhooks() {
  const [hooks, setHooks] = useState<Webhook[]>([]);
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setHooks((await api.webhooks()).webhooks || []);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load");
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.createWebhook(url.trim(), secret.trim());
      setUrl("");
      setSecret("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add webhook");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    setBusy(true);
    try {
      await api.deleteWebhook(id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not delete");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>Webhooks</h1>
        <p>POST platform events (contributions, payouts, defaults, KYC) to your endpoints. Payloads are signed with HMAC-SHA256 in <span className="mono">X-Rota-Signature</span>.</p>
      </div>
      {error && <div className="error" style={{ marginBottom: 14 }}>{error}</div>}

      <div className="section-card">
        <h3>Add endpoint</h3>
        <form className="form" onSubmit={add} style={{ maxWidth: "100%" }}>
          <div className="toolbar" style={{ marginBottom: 0 }}>
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://your-service.com/hooks/rota" style={{ flex: 2, minWidth: 280 }} />
            <input value={secret} onChange={(e) => setSecret(e.target.value)} placeholder="Signing secret (optional)" style={{ flex: 1, minWidth: 180 }} />
            <button className="btn btn-primary" type="submit" disabled={busy || !url.trim()}>Add webhook</button>
          </div>
        </form>
      </div>

      <div className="section-card">
        <h3>Registered endpoints</h3>
        {hooks.length === 0 ? (
          <div className="empty" style={{ padding: "20px 0" }}>No webhooks yet.</div>
        ) : (
          <table>
            <thead><tr><th>URL</th><th>Status</th><th>Added</th><th style={{ textAlign: "right" }}></th></tr></thead>
            <tbody>
              {hooks.map((h) => (
                <tr key={h.id}>
                  <td className="mono">{h.url}</td>
                  <td><span className={`badge ${h.active ? "ok" : ""}`}>{h.active ? "active" : "paused"}</span></td>
                  <td className="muted">{new Date(h.created_at).toLocaleDateString()}</td>
                  <td style={{ textAlign: "right" }}>
                    <button className="btn danger" disabled={busy} onClick={() => remove(h.id)}>Delete</button>
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
