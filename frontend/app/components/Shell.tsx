"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api, currentUser, isAuthed, logout, type User } from "@/lib/api";

const MEMBER_NAV = [
  { href: "/dashboard", label: "Overview" },
  { href: "/dashboard/browse", label: "Browse groups" },
  { href: "/dashboard/transactions", label: "Transactions" },
];

const ADMIN_NAV = [
  { href: "/admin", label: "Overview" },
  { href: "/admin/users", label: "Users" },
  { href: "/admin/groups", label: "Groups" },
  { href: "/admin/transactions", label: "Transactions" },
  { href: "/admin/kyc", label: "KYC review" },
  { href: "/admin/webhooks", label: "Webhooks" },
  { href: "/admin/audit", label: "Audit log" },
];

export default function Shell({ role, children }: { role: "member" | "admin"; children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<User | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function init() {
      if (!isAuthed()) {
        router.replace("/login");
        return;
      }
      let u = currentUser();
      if (!u) {
        try {
          u = await api.me();
          localStorage.setItem("me", JSON.stringify(u));
        } catch {
          logout();
          router.replace("/login");
          return;
        }
      }
      if (cancelled) return;
      if (role === "admin" && u.role !== "admin") {
        router.replace("/dashboard");
        return;
      }
      setUser(u);
    }
    init();
    return () => {
      cancelled = true;
    };
  }, [role, router]);

  // Close the drawer on navigation.
  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  if (!user) return null;

  const nav = role === "admin" ? ADMIN_NAV : MEMBER_NAV;
  const initial = (user.display_name || user.email || "?").charAt(0).toUpperCase();
  const home = role === "admin" ? "/admin" : "/dashboard";

  return (
    <div className="shell">
      {/* Mobile top bar (hidden on desktop) */}
      <div className="mobile-topbar">
        <Link href={home} className="brand">Rotasavings</Link>
        <button className="hamburger" aria-label="Menu" aria-expanded={menuOpen} onClick={() => setMenuOpen((v) => !v)}>
          {menuOpen ? (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          ) : (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M3 6h18M3 12h18M3 18h18" /></svg>
          )}
        </button>
      </div>

      {menuOpen && <div className="drawer-overlay" onClick={() => setMenuOpen(false)} />}

      <aside className={`sidebar ${menuOpen ? "open" : ""}`}>
        <Link href={home} className="brand sidebar-brand">Rotasavings</Link>
        <nav className="side-nav">
          {nav.map((n) => {
            const active = pathname === n.href || (n.href !== "/dashboard" && n.href !== "/admin" && pathname.startsWith(n.href));
            return (
              <Link key={n.href} href={n.href} className={active ? "active" : ""} onClick={() => setMenuOpen(false)}>
                {n.label}
              </Link>
            );
          })}
        </nav>
        <div className="side-foot">
          <div className="user-chip">
            <span className="ava">{initial}</span>
            <div>
              <div style={{ fontWeight: 700 }}>{user.display_name}</div>
              <div className="role-pill">{user.role}</div>
            </div>
          </div>
          <button className="btn btn-ghost btn-block" style={{ marginTop: 8 }} onClick={() => { logout(); router.replace("/login"); }}>
            Sign out
          </button>
        </div>
      </aside>

      <main className="main">{children}</main>
    </div>
  );
}
