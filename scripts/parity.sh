#!/usr/bin/env bash
# Compares the Express API (A) and the Go API (B) running against the SAME database.
# Usage: scripts/parity.sh http://localhost:3098 http://localhost:3097
# Both servers must share JWT_SECRET and have the seeded admin user (admin / admin123).
#
# Phases:
#   1. read-only GETs                      → status + JSON must be identical
#   2. requests that fail validation/auth  → status + JSON identical (no data changes)
#   3. cross-compat: login on A, use the JWT on B (and vice versa); token from A used on B
#   4. writes done through B are read back through A, and the reverse
set -uo pipefail

A=${1:?express base url}
B=${2:?go base url}
pass=0
fail=0

norm() { jq -S . 2>/dev/null || cat; }
# Express leaves the order of rows with equal sort keys unspecified; Go breaks ties by id.
# Use UNORDERED=1 for lists that may contain such ties.
normu() { jq -S 'if type == "array" then sort_by(.id) else . end' 2>/dev/null || cat; }

# compare METHOD PATH [BODY] [AUTH_HEADER]
compare() {
  local method=$1 path=$2 body=${3:-} auth=${4:-}
  local args=(-s -o /dev/stdout -w '\n%{http_code}' -X "$method" -H 'Content-Type: application/json')
  [[ -n $auth ]] && args+=(-H "Authorization: $auth")
  [[ -n $body ]] && args+=(--data "$body")
  local ra rb sa sb
  ra=$(curl "${args[@]}" "$A$path"); sa=${ra##*$'\n'}; ra=${ra%$'\n'*}
  rb=$(curl "${args[@]}" "$B$path"); sb=${rb##*$'\n'}; rb=${rb%$'\n'*}
  local n=norm
  [[ ${UNORDERED:-0} == 1 ]] && n=normu
  if [[ $sa == "$sb" && "$($n <<<"$ra")" == "$($n <<<"$rb")" ]]; then
    pass=$((pass + 1))
    printf '  ok    %-6s %-45s %s\n' "$method" "$path" "$sa"
  else
    fail=$((fail + 1))
    printf '  DIFF  %-6s %-45s express=%s go=%s\n' "$method" "$path" "$sa" "$sb"
    diff <(norm <<<"$ra") <(norm <<<"$rb") | head -20 | sed 's/^/        /'
  fi
}

check() { # check DESCRIPTION CONDITION_EXIT_CODE
  if [[ $2 -eq 0 ]]; then pass=$((pass + 1)); echo "  ok    $1"; else fail=$((fail + 1)); echo "  FAIL  $1"; fi
}

PARITY_USER=${PARITY_USER:-admin}
PARITY_PASS=${PARITY_PASS:-admin123}
login() { curl -s -X POST -H "Content-Type: application/json" --data "{\"username\":\"$PARITY_USER\",\"password\":\"$PARITY_PASS\"}" "$1/api/auth/login" | jq -r .token; }

echo "== 1. public + admin reads"
TA=$(login "$A")
for p in /api/health /api/projects /api/testimonials /api/blog /api/org-experience /api/site-content \
  /api/projects/cak-mad-editor /api/projects/does-not-exist; do
  compare GET "$p"
done
for p in /api/projects/admin-list /api/testimonials/admin-list /api/blog/admin-list /api/org-experience/admin-list; do
  compare GET "$p" "" "Bearer $TA"
done

echo "== 2. validation and auth errors (no state change)"
compare POST /api/auth/login '{}'
compare POST /api/auth/login '{"username":"admin","password":"wrong"}'
compare GET /api/auth/me
compare GET /api/auth/me "" "Bearer not-a-jwt"
compare GET /api/projects/admin-list
compare POST /api/projects '{"title":"x"}' "Bearer $TA"
compare POST /api/projects '{"title":"x","description":"y","category":"mobile"}' "Bearer $TA"
compare POST /api/projects '{"title":"x","description":"y","highlights":[1]}' "Bearer $TA"
compare PUT /api/projects/abc '{}' "Bearer $TA"
compare PUT /api/projects/999999 '{"title":"x"}' "Bearer $TA"
compare PUT /api/projects/reorder '{"orderedIds":"nope"}' "Bearer $TA"
compare DELETE /api/projects/999999 "" "Bearer $TA"
compare POST /api/testimonials '{"name":"a"}' "Bearer $TA"
compare DELETE /api/testimonials/999999 "" "Bearer $TA"
compare POST /api/blog '{}' "Bearer $TA"
compare PUT /api/blog/xyz '{}' "Bearer $TA"
compare POST /api/org-experience '{"period":"2026"}' "Bearer $TA"
compare POST /api/org-experience '{"period":"2026","organization":"o","role":"r","type":"club"}' "Bearer $TA"
compare PUT /api/org-experience/1.5 '{}' "Bearer $TA"
compare PUT /api/site-content/unknown '{}' "Bearer $TA"
compare PUT /api/site-content/contact '{"ctaTitle":""}' "Bearer $TA"
compare PUT /api/site-content/about '{"eyebrow":"e","name":"n","paragraphs":[],"summaryTitle":"s","summary":[]}' "Bearer $TA"
compare POST /api/admin/tokens '{"name":""}' "Bearer $TA"
compare POST /api/admin/tokens '{"name":"x","scope":"admin"}' "Bearer $TA"
compare DELETE /api/admin/tokens/999999 "" "Bearer $TA"

echo "== 3. credential cross-compatibility"
TB=$(login "$B")
curl -sf -H "Authorization: Bearer $TA" "$B/api/auth/me" | jq -e --arg u "$PARITY_USER" '.username==$u' >/dev/null
check "JWT issued by Express accepted by Go" $?
curl -sf -H "Authorization: Bearer $TB" "$A/api/auth/me" | jq -e --arg u "$PARITY_USER" '.username==$u' >/dev/null
check "JWT issued by Go accepted by Express" $?
PF=$(curl -s -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $TA" --data '{"name":"parity","scope":"read"}' "$A/api/admin/tokens" | jq -r .token)
compare GET /api/projects/admin-list "" "Bearer $PF"
compare POST /api/projects '{"title":"x","description":"y"}' "Bearer $PF"
compare GET /api/admin/tokens "" "Bearer $TA"
TID=$(curl -s -H "Authorization: Bearer $TA" "$A/api/admin/tokens" | jq '.[0].id')
curl -s -X DELETE -H "Authorization: Bearer $TB" "$B/api/admin/tokens/$TID" >/dev/null
compare GET /api/projects/admin-list "" "Bearer $PF"

echo "== 4. writes through one API, reads through the other"
NEW=$(curl -s -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $TB" \
  --data '{"title":"Parity Check","description":"made by go","tags":["go"],"highlights":[" a ",""],"featured":false}' "$B/api/projects")
SLUG=$(jq -r .slug <<<"$NEW"); NID=$(jq -r .id <<<"$NEW")
[[ $SLUG == parity-check ]]; check "Go auto-slug = parity-check" $?
compare GET "/api/projects/$SLUG"
curl -s -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $TA" \
  --data '{"title":"Parity Check","description":"made by express"}' "$A/api/projects" >/dev/null
compare GET /api/projects/parity-check-2
compare POST /api/projects '{"title":"z","description":"z","slug":"parity-check"}' "Bearer $TA"
curl -s -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $TA" \
  --data '{"year":"","role":"Tester","visible":false}' "$A/api/projects/$NID" >/dev/null
UNORDERED=1 compare GET /api/projects/admin-list "" "Bearer $TA"
IDS=$(curl -s -H "Authorization: Bearer $TA" "$A/api/projects/admin-list" | jq -c '[.[].id] | reverse')
curl -s -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $TB" --data "{\"orderedIds\":$IDS}" "$B/api/projects/reorder" >/dev/null
compare GET /api/projects
SC='{"ctaTitle":"Hi","ctaSubtitle":"s","brandDesc":"b","email":"a@b.co","location":"Jogja","github":"https://github.com/x","linkedin":"https://linkedin.com/in/x","x":"https://x.com/x","instagram":"https://instagram.com/x","extra":"dropped"}'
curl -s -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer $TB" --data "$SC" "$B/api/site-content/contact" >/dev/null
compare GET /api/site-content
curl -s -X DELETE -H "Authorization: Bearer $TA" "$A/api/projects/$NID" >/dev/null
compare GET "/api/projects/$SLUG"

echo
echo "parity: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
