# ROTASAVINGS Admin (Next.js)

Operator dashboard for the ROTASAVINGS backend: KPIs, KYC review queue,
liquidity-stress board, user management (suspend/activate), and the audit trail.
It is a client-rendered Next.js 15 (App Router) app that talks to the Go API.

## Run

```bash
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev   # http://localhost:3000
```

Sign in with the seeded admin (default `admin@rotasavings.local` /
`changeme123`). Access tokens are stored in `localStorage` and refreshed
transparently via `/v1/auth/refresh` on a 401.

## Build

```bash
npm run build
```

## Layout

| Path                      | Purpose                                                  |
|---------------------------|----------------------------------------------------------|
| `app/page.tsx`            | Marketing landing page (hero, how-it-works, features)    |
| `app/login/page.tsx`      | Operator sign-in                                         |
| `app/admin/page.tsx`      | Dashboard (overview, KYC, liquidity, users, audit)       |
| `app/components/`         | Shared `SiteHeader` / `SiteFooter`                       |
| `app/globals.css`         | Design system (landing + dashboard)                      |
| `lib/api.ts`              | Typed API client with token refresh                      |

Routes: `/` landing, `/login` sign-in, `/admin` dashboard (redirects to
`/login` when unauthenticated).
