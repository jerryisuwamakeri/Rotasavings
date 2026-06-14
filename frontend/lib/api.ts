// Typed client for the ROTASAVINGS API. Tokens + the current user are kept in
// localStorage; the access token is refreshed transparently on a 401.

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

/* ---------- types ---------- */

export type Role = "member" | "admin";

export type User = {
  id: string;
  email: string;
  display_name: string;
  wallet_address: string;
  kyc_status: "pending" | "approved" | "rejected";
  role: Role;
  status: "active" | "suspended";
};

export type GroupState = "CREATED" | "ACTIVE" | "SETTLEMENT" | "CLOSED";

export type Group = {
  id: string;
  name: string;
  organizer_id: string;
  contribution_amount: number;
  cycle_length: string;
  max_members: number;
  total_cycles: number;
  members: string[];
  payout_order: string[];
  state: GroupState;
  contract_address: string;
  created_at: string;
};

export type MyGroup = { group: Group; organizer: boolean; status: string };

export type Membership = {
  id: string;
  group_id: string;
  user_id: string;
  organizer: boolean;
  status: string;
};

export type Cycle = {
  group_id: string;
  index: number;
  deadline: string;
  payout_user: string;
  settled: boolean;
};

export type CycleStatus = {
  cycle: Cycle;
  members: { user_id: string; paid: boolean }[];
  collected: number;
  expected: number;
};

export type JoinRequest = {
  id: string;
  group_id: string;
  user_id: string;
  status: "pending" | "approved" | "rejected";
  created_at: string;
};

export type Invitation = {
  id: string;
  group_id: string;
  user_id: string;
  status: "pending" | "accepted" | "declined";
};

export type Reputation = {
  user_id: string;
  contributions_made: number;
  contributions_missed: number;
  payouts_received: number;
  total_contributed: number;
  total_missed: number;
  reliability_ratio: number;
};

export type Transaction = {
  id: string;
  user_id: string;
  group_id: string;
  cycle_index: number;
  type: "ContributionMade" | "ContributionMissed" | "PayoutReceived" | "GroupExit" | "GroupExpulsion";
  amount: number;
  timestamp: string;
};

export type Notification = {
  id: string;
  kind: string;
  title: string;
  body: string;
  read: boolean;
  created_at: string;
};

export type Overview = {
  users: number;
  groups_by_state: Record<string, number>;
  total_groups: number;
  pending_kyc: number;
  total_escrow: number;
  total_payouts: number;
};

export type AuditEntry = { id: string; actor_id: string; action: string; target: string; created_at: string };
export type Webhook = { id: string; url: string; active: boolean; created_at: string };
export type UserDetail = { user: User; groups: MyGroup[]; reputation: Reputation };
export type Liquidity = {
  group_id: string;
  cycle_index: number;
  expected_inflow: number;
  predicted_shortfall: number;
  escrow_balance: number;
  collapse_probability: number;
  level: "ok" | "stressed" | "critical";
};

/* ---------- token + user storage ---------- */

function ls(key: string): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(key) || "";
}

function setTokens(access: string, refresh: string) {
  localStorage.setItem("access_token", access);
  localStorage.setItem("refresh_token", refresh);
}

export function logout() {
  if (typeof window === "undefined") return;
  localStorage.removeItem("access_token");
  localStorage.removeItem("refresh_token");
  localStorage.removeItem("me");
}

export function isAuthed() {
  return !!ls("access_token");
}

export function currentUser(): User | null {
  const raw = ls("me");
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

async function refresh(): Promise<boolean> {
  const token = ls("refresh_token");
  if (!token) return false;
  const res = await fetch(`${BASE}/v1/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: token }),
  });
  if (!res.ok) return false;
  const data = await res.json();
  setTokens(data.access_token, data.refresh_token);
  return true;
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(ls("access_token") ? { Authorization: `Bearer ${ls("access_token")}` } : {}),
      ...(init.headers || {}),
    },
  });
  if (res.status === 401 && retry && (await refresh())) {
    return request<T>(path, init, false);
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/* ---------- auth ---------- */

export async function register(input: {
  email: string;
  password: string;
  display_name: string;
  wallet_address: string;
}): Promise<User> {
  const res = await fetch(`${BASE}/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...input, kyc_provider: "self", kyc_signature: "demo" }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({ error: "Registration failed" }));
    throw new Error(b.error || "Registration failed");
  }
  return res.json();
}

export async function login(email: string, password: string): Promise<User> {
  const res = await fetch(`${BASE}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error("Invalid email or password");
  const data = await res.json();
  setTokens(data.access_token, data.refresh_token);
  localStorage.setItem("me", JSON.stringify(data.user));
  return data.user as User;
}

/* ---------- api surface ---------- */

export const api = {
  // me
  me: () => request<User>("/v1/me"),
  updateMe: (display_name: string) =>
    request<User>("/v1/me", { method: "PATCH", body: JSON.stringify({ display_name }) }),
  myGroups: () => request<{ groups: MyGroup[] }>("/v1/me/groups"),
  myTransactions: () => request<{ transactions: Transaction[] }>("/v1/me/transactions"),
  notifications: () => request<{ notifications: Notification[] }>("/v1/me/notifications"),
  invitations: () => request<{ invitations: Invitation[] }>("/v1/me/invitations"),
  reputation: (userId: string) => request<Reputation>(`/v1/users/${userId}/reputation`),

  // groups
  groups: (limit = 100) => request<{ groups: Group[]; total: number }>(`/v1/groups?limit=${limit}`),
  group: (id: string) => request<Group>(`/v1/groups/${id}`),
  createGroup: (input: { name: string; contribution_amount: number; cycle_length: string; max_members: number }) =>
    request<Group>("/v1/groups", { method: "POST", body: JSON.stringify(input) }),
  members: (id: string) => request<{ members: Membership[] }>(`/v1/groups/${id}/members`),
  cycles: (id: string) => request<{ cycles: Cycle[] }>(`/v1/groups/${id}/cycles`),
  cycleStatus: (id: string, idx: number) => request<CycleStatus>(`/v1/groups/${id}/cycles/${idx}/status`),

  // group actions (CRUD-ish)
  requestJoin: (id: string) => request<JoinRequest>(`/v1/groups/${id}/join-requests`, { method: "POST" }),
  listJoinRequests: (id: string) => request<{ join_requests: JoinRequest[] }>(`/v1/groups/${id}/join-requests`),
  decideJoin: (reqId: string, approve: boolean) =>
    request<JoinRequest>(`/v1/join-requests/${reqId}/decision`, { method: "POST", body: JSON.stringify({ approve }) }),
  invite: (id: string, userId: string) =>
    request<Invitation>(`/v1/groups/${id}/invitations`, { method: "POST", body: JSON.stringify({ user_id: userId }) }),
  respondInvite: (invId: string, accept: boolean) =>
    request<Invitation>(`/v1/invitations/${invId}/response`, { method: "POST", body: JSON.stringify({ accept }) }),
  leave: (id: string) => request<{ status: string }>(`/v1/groups/${id}/leave`, { method: "POST" }),
  removeMember: (id: string, userId: string) =>
    request<{ status: string }>(`/v1/groups/${id}/members/${userId}`, { method: "DELETE" }),
  activate: (id: string, payout_order?: string[]) =>
    request<Group>(`/v1/groups/${id}/activate`, { method: "POST", body: JSON.stringify({ payout_order: payout_order || [] }) }),
  contribute: (id: string, cycle_index: number, source: string) =>
    request<unknown>(`/v1/groups/${id}/contributions`, {
      method: "POST",
      headers: { "Idempotency-Key": `${id}-${cycle_index}-${Date.now()}` },
      body: JSON.stringify({ cycle_index, source }),
    }),
  settle: (id: string, idx: number) => request<unknown>(`/v1/groups/${id}/cycles/${idx}/settle`, { method: "POST" }),

  // admin
  overview: () => request<Overview>("/v1/admin/overview"),
  adminUsers: () => request<{ users: User[]; total: number }>("/v1/admin/users?limit=100"),
  pendingKYC: () => request<{ pending: User[] }>("/v1/admin/kyc/pending"),
  decideKYC: (id: string, approve: boolean) =>
    request<User>(`/v1/admin/kyc/${id}/decision`, { method: "POST", body: JSON.stringify({ approve }) }),
  suspend: (id: string) => request<User>(`/v1/admin/users/${id}/suspend`, { method: "POST" }),
  activateUser: (id: string) => request<User>(`/v1/admin/users/${id}/activate`, { method: "POST" }),
  liquidity: () => request<{ groups: Liquidity[] }>("/v1/admin/liquidity"),
  audit: () => request<{ audit: AuditEntry[]; total: number }>("/v1/admin/audit?limit=50"),
  // admin: extended controls
  adminUserDetail: (id: string) => request<UserDetail>(`/v1/admin/users/${id}`),
  setRole: (id: string, role: Role) =>
    request<User>(`/v1/admin/users/${id}/role`, { method: "POST", body: JSON.stringify({ role }) }),
  adminGroups: () => request<{ groups: Group[]; total: number }>("/v1/admin/groups?limit=200"),
  adminTransactions: () => request<{ transactions: Transaction[]; total: number }>("/v1/admin/transactions?limit=100"),
  forceSettle: (groupId: string, idx: number) =>
    request<unknown>(`/v1/admin/groups/${groupId}/cycles/${idx}/settle`, { method: "POST" }),
  webhooks: () => request<{ webhooks: Webhook[] }>("/v1/admin/webhooks"),
  createWebhook: (url: string, secret: string) =>
    request<Webhook>("/v1/admin/webhooks", { method: "POST", body: JSON.stringify({ url, secret }) }),
  deleteWebhook: (id: string) => request<unknown>(`/v1/admin/webhooks/${id}`, { method: "DELETE" }),
};

/* ---------- helpers ---------- */

// money formats minor units (kobo) as naira.
export function money(minor: number): string {
  return "₦" + (minor / 100).toLocaleString(undefined, { maximumFractionDigits: 0 });
}
