#!/usr/bin/env bash
# Seed a known set of demo accounts against a running backend, so you have
# reliable logins across roles. Idempotent-ish: re-registering an existing email
# just fails with 409 and is ignored.
#
# Usage:  ./scripts/seed.sh            (defaults to http://localhost:8080)
#         API=http://host:8080 ./scripts/seed.sh
#
# Passwords are NOT stored anywhere readable - they are PBKDF2-hashed on the
# server. The plaintext values below are the demo passwords this script SETS.

set -euo pipefail
API="${API:-http://localhost:8080}"
ADMIN_EMAIL="${ROTA_ADMIN_EMAIL:-admin@rotasavings.local}"
ADMIN_PASS="${ROTA_ADMIN_PASSWORD:-changeme123}"
PASS="password123"

jqid() { sed -n 's/.*"id":"\([^"]*\)".*/\1/p'; }
tok()  { sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'; }

echo "Seeding against $API"
ADM=$(curl -s -X POST "$API/v1/auth/login" -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" | tok)
if [ -z "$ADM" ]; then echo "ERROR: could not log in as admin ($ADMIN_EMAIL). Is the backend running?"; exit 1; fi

# register helper -> echoes the user id (existing users are looked up via admin list)
register() {
  local email="$1" name="$2" wallet="$3"
  local id
  id=$(curl -s -X POST "$API/v1/auth/register" \
        -d "{\"email\":\"$email\",\"password\":\"$PASS\",\"display_name\":\"$name\",\"wallet_address\":\"$wallet\",\"kyc_provider\":\"self\",\"kyc_signature\":\"seed\"}" | jqid)
  echo "$id"
}

approve_kyc() { curl -s -o /dev/null -X POST "$API/v1/admin/kyc/$1/decision" -H "Authorization: Bearer $ADM" -d '{"approve":true}'; }
make_admin()  { curl -s -o /dev/null -X POST "$API/v1/admin/users/$1/role" -H "Authorization: Bearer $ADM" -d '{"role":"admin"}'; }

AMARA=$(register "amara@x.com" "Amara Okafor" "0xAmara"); [ -n "$AMARA" ] && approve_kyc "$AMARA"
BOLA=$(register  "bola@x.com"  "Bola Ade"     "0xBola");  [ -n "$BOLA" ]  && approve_kyc "$BOLA"
CHIDI=$(register "chidi@x.com" "Chidi Eze"    "0xChidi") # left pending on purpose
OPS=$(register   "ops@rotasavings.local" "Ops Manager" "0xOps"); [ -n "$OPS" ] && { approve_kyc "$OPS"; make_admin "$OPS"; }

cat <<EOF

Done. Login credentials (passwords are what this script SET; the server stores only hashes):

  ADMIN
    $ADMIN_EMAIL / $ADMIN_PASS        (seeded on boot)
    ops@rotasavings.local / $PASS     (promoted to admin)

  MEMBERS
    amara@x.com / $PASS               (KYC approved)
    bola@x.com  / $PASS               (KYC approved)
    chidi@x.com / $PASS               (KYC pending - shows in the review queue)

Note: with ROTA_DB_PATH=memory these vanish on restart. Use the default SQLite
DB (ROTA_DB_PATH=rota.db) to keep them across restarts.
EOF
