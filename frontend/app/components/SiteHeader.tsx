"use client";

import Link from "next/link";
import { useState } from "react";

export default function SiteHeader() {
  const [open, setOpen] = useState(false);
  return (
    <header className="site-header">
      <div className="container inner">
        <Link href="/" className="brand">Rotasavings</Link>

        <nav className="nav-links hide-sm">
          <a href="/#how">How it works</a>
          <a href="/#features">Features</a>
          <a href="/#security">Security</a>
          <Link href="/login" className="btn btn-ghost btn-sm">Sign in</Link>
        </nav>

        <button className="hamburger show-sm" aria-label="Menu" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
          {open ? (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
          ) : (
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M3 6h18M3 12h18M3 18h18" /></svg>
          )}
        </button>
      </div>

      {open && (
        <div className="mobile-menu">
          <a href="/#how" onClick={() => setOpen(false)}>How it works</a>
          <a href="/#features" onClick={() => setOpen(false)}>Features</a>
          <a href="/#security" onClick={() => setOpen(false)}>Security</a>
          <Link href="/login" className="btn btn-primary btn-block" onClick={() => setOpen(false)}>Sign in</Link>
        </div>
      )}
    </header>
  );
}
