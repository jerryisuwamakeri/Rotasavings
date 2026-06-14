"use client";

// Lightweight, dependency-free charts in the brand green, with tinted shades
// for multi-segment differentiation.

const SHADES = ["#10c45f", "#0aa850", "#7bd9a4", "#bdeccf", "#e7f8ee"];
const GREEN = "#10c45f";
const TRACK = "#eef1ee";
const FG = "#14181b";

export function ChartCard({
  title,
  big,
  children,
}: {
  title: string;
  big?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="chart-card">
      <div className="ch-head">
        <span className="ch-title">{title}</span>
        {big !== undefined && <span className="ch-big">{big}</span>}
      </div>
      {children}
    </div>
  );
}

// Vertical bar chart. data: [{label, value}]. Bars are solid black.
export function BarChart({
  data,
  height = 120,
  format = (n: number) => String(n),
}: {
  data: { label: string; value: number }[];
  height?: number;
  format?: (n: number) => string;
}) {
  if (data.length === 0) return <div className="empty" style={{ padding: "26px 0" }}>No data yet.</div>;
  const max = Math.max(1, ...data.map((d) => d.value));
  return (
    <div className="bars" style={{ height }}>
      {data.map((d, i) => (
        <div key={i} className="col" title={`${d.label}: ${format(d.value)}`}>
          <div className={`bar ${d.value === 0 ? "ghost" : ""}`} style={{ height: `${(d.value / max) * 100}%` }} />
          <span className="blabel">{d.label}</span>
        </div>
      ))}
    </div>
  );
}

// Donut with grayscale segments + a legend. data: [{label, value}].
export function Donut({
  data,
  centerLabel,
  centerValue,
}: {
  data: { label: string; value: number }[];
  centerLabel?: string;
  centerValue?: string;
}) {
  const total = data.reduce((s, d) => s + d.value, 0);
  const r = 42;
  const c = 2 * Math.PI * r;
  let offset = 0;
  return (
    <div className="ring-wrap">
      <svg viewBox="0 0 100 100" width="120" height="120" style={{ flex: "none" }}>
        <circle cx="50" cy="50" r={r} fill="none" stroke={TRACK} strokeWidth="14" />
        {total > 0 &&
          data.map((d, i) => {
            const dash = (d.value / total) * c;
            const el = (
              <circle
                key={i}
                cx="50"
                cy="50"
                r={r}
                fill="none"
                stroke={SHADES[i % SHADES.length]}
                strokeWidth="14"
                strokeDasharray={`${dash} ${c - dash}`}
                strokeDashoffset={-offset}
                transform="rotate(-90 50 50)"
              />
            );
            offset += dash;
            return el;
          })}
        {centerValue !== undefined && (
          <text x="50" y="50" textAnchor="middle" dominantBaseline="central" fontSize="18" fontWeight="700" fill={FG}>
            {centerValue}
          </text>
        )}
        {centerLabel && (
          <text x="50" y="63" textAnchor="middle" fontSize="7" fill="#99a1a8" fontFamily="monospace">
            {centerLabel}
          </text>
        )}
      </svg>
      <div className="legend" style={{ flexDirection: "column", gap: 8, marginTop: 0 }}>
        {data.map((d, i) => (
          <span key={i} className="key">
            <span className="sw" style={{ background: SHADES[i % SHADES.length], borderColor: SHADES[i % SHADES.length] }} />
            {d.label} <b style={{ color: "var(--fg)", marginLeft: 4 }}>{d.value}</b>
          </span>
        ))}
      </div>
    </div>
  );
}

// Single-value progress ring (0..1).
export function Ring({ value, label }: { value: number; label?: string }) {
  const r = 42;
  const c = 2 * Math.PI * r;
  const v = Math.max(0, Math.min(1, value));
  return (
    <div className="ring-wrap">
      <svg viewBox="0 0 100 100" width="110" height="110" style={{ flex: "none" }}>
        <circle cx="50" cy="50" r={r} fill="none" stroke={TRACK} strokeWidth="12" />
        <circle
          cx="50"
          cy="50"
          r={r}
          fill="none"
          stroke={GREEN}
          strokeWidth="12"
          strokeLinecap="round"
          strokeDasharray={`${v * c} ${c}`}
          transform="rotate(-90 50 50)"
        />
        <text x="50" y="50" textAnchor="middle" dominantBaseline="central" fontSize="20" fontWeight="700" fill={FG}>
          {Math.round(v * 100)}%
        </text>
      </svg>
      {label && <div className="muted mono" style={{ fontSize: 12, textTransform: "uppercase", letterSpacing: "0.08em" }}>{label}</div>}
    </div>
  );
}
