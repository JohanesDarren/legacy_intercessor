#!/usr/bin/env bash
# End-to-end smoke test for the Legacy Intercessor API.
# Usage: bash scripts/smoke.sh [base_url]
set -uo pipefail

BASE="${1:-http://localhost:8080/api/v1}"
EMAIL="admin@legacy.sukawarna.id"
PASSWORD="LegacyIntercessor#2026"

pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; exit 1; }
step() { echo; echo "== $1"; }

step "1. Health"
curl -sf "$BASE/health" >/dev/null && pass "health endpoint" || fail "health endpoint"

step "2. Login as admin"
LOGIN=$(curl -sf -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
TOKEN=$(echo "$LOGIN" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["access_token"])')
[ -n "$TOKEN" ] && pass "got access token" || fail "no access token"
AUTH="Authorization: Bearer $TOKEN"

step "3. Reject a bad password"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"wrong-password\"}")
[ "$CODE" = "401" ] && pass "401 on bad password" || fail "expected 401, got $CODE"

step "4. Reject a viewer reading prayer requests (RBAC)"
VIEWER=$(curl -sf -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"media@legacy.sukawarna.id","password":"LegacyIntercessor#2026"}')
VTOKEN=$(echo "$VIEWER" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["access_token"])')
CODE=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/prayer-requests" -H "Authorization: Bearer $VTOKEN")
[ "$CODE" = "403" ] && pass "403 for viewer on prayer-requests" || fail "expected 403, got $CODE"

step "5. Next service"
SVC=$(curl -sf "$BASE/services/next" -H "$AUTH")
SVC_ID=$(echo "$SVC" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
SVC_DATE=$(echo "$SVC" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["starts_at"][:10])')
pass "next service $SVC_ID on $SVC_DATE"

step "6. Categories"
CATS=$(curl -sf "$BASE/categories?kind=prayer" -H "$AUTH")
HEALTH_CAT=$(echo "$CATS" | python -c 'import sys,json; d=json.load(sys.stdin)["data"]; print([c["id"] for c in d if c["name"]=="Health"][0])')
FAMILY_CAT=$(echo "$CATS" | python -c 'import sys,json; d=json.load(sys.stdin)["data"]; print([c["id"] for c in d if c["name"]=="Family"][0])')
pass "prayer categories loaded"

step "7. Create prayer requests (mixed consent)"
mk_prayer() {
  curl -sf -X POST "$BASE/prayer-requests" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"service_id\":\"$SVC_ID\",\"category_id\":\"$1\",\"requester_name\":\"$2\",
         \"requester_gender\":\"$3\",\"title\":\"$4\",\"content\":\"$5\",
         \"is_urgent\":$6,\"display_consent\":\"$7\"}"
}
P1=$(mk_prayer "$HEALTH_CAT" "Sari" female "Healing for post-surgery recovery" "Sari had surgery last week and asks for prayer for a smooth recovery and strength for her family. Contact 0812-3456-7890." true yes)
P1_ID=$(echo "$P1" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
P2=$(mk_prayer "$HEALTH_CAT" "Budi" male "Strength during chemotherapy" "Budi is in his third round of chemotherapy and asks for strength and peace." false anonymous)
P3=$(mk_prayer "$FAMILY_CAT" "Rina" female "Restoration of marriage" "Rina asks prayer for healing and restoration in her marriage." false no)
P4=$(mk_prayer "$FAMILY_CAT" "Anonymous" unspecified "Peace for a difficult family situation" "A member asks for peace and wisdom in a difficult family situation." false anonymous)
pass "4 prayer requests created"

step "8. Urgent request sorts first"
FIRST=$(curl -sf "$BASE/prayer-requests?sort_by=is_urgent&limit=5" -H "$AUTH" | python -c 'import sys,json; print(json.load(sys.stdin)["data"][0]["is_urgent"])')
[ "$FIRST" = "True" ] && pass "urgent request sorted first" || fail "urgent did not sort first"

step "9. Search works"
COUNT=$(curl -sf "$BASE/prayer-requests?q=chemotherapy" -H "$AUTH" | python -c 'import sys,json; print(json.load(sys.stdin)["meta"]["total"])')
[ "$COUNT" = "1" ] && pass "search found 1 match" || fail "expected 1 search match, got $COUNT"

step "10. Filter by consent=no"
COUNT=$(curl -sf "$BASE/prayer-requests?consent=no" -H "$AUTH" | python -c 'import sys,json; print(json.load(sys.stdin)["meta"]["total"])')
[ "$COUNT" = "1" ] && pass "consent=no filter returns 1" || fail "expected 1, got $COUNT"

step "11. Duplicate warning"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/prayer-requests" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"service_id\":\"$SVC_ID\",\"category_id\":\"$HEALTH_CAT\",\"requester_name\":\"Sari\",
       \"title\":\"Healing for post-surgery recovery\",\"content\":\"Same person, similar title.\"}")
[ "$CODE" = "409" ] && pass "409 duplicate warning" || fail "expected 409, got $CODE"

step "12. Log a follow-up on the urgent request"
FU=$(curl -sf -X POST "$BASE/prayer-requests/$P1_ID/follow-ups" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"channel\":\"visit\",\"update_type\":\"current_condition\",\"condition_status\":\"improving\",
       \"notes\":\"Visited Sari at home. She is recovering well and grateful for the prayer.\",
       \"next_action\":\"Call again next week\",\"next_action_date\":\"$SVC_DATE\"}")
FU_ID=$(echo "$FU" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
[ -n "$FU_ID" ] && pass "follow-up logged" || fail "follow-up failed"

step "13. AI summary (offline provider) - only consented records"
SUM=$(curl -sf -X POST "$BASE/ai/summaries" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"service_id\":\"$SVC_ID\",\"kind\":\"prayer\",\"language\":\"en\",\"max_words_per_item\":40}")
NSUM=$(echo "$SUM" | python -c 'import sys,json; print(len(json.load(sys.stdin)["data"]))')
[ "$NSUM" -ge 2 ] && pass "$NSUM draft summaries created (Health + Family)" || fail "expected >=2 summaries, got $NSUM"

# Every summary must list source ids and be a draft.
echo "$SUM" | python -c '
import sys,json
d=json.load(sys.stdin)["data"]
assert all(s["status"]=="draft" for s in d), "not all drafts"
assert all(len(s["source_ids"])>0 for s in d), "missing source ids"
print("  PASS  all summaries are drafts with source ids")
'

# The consent=no record must never appear as a source.
echo "$SUM" | python -c "
import sys,json
d=json.load(sys.stdin)['data']
ids=[i for s in d for i in s['source_ids']]
assert '$P3_ID' not in ids, 'consent=no record leaked into the summary'
print('  PASS  consent=no record excluded from AI sources')
"

step "14. Export is blocked while a summary is still a draft"
PRES=$(curl -sf -X POST "$BASE/presentations" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"service_id\":\"$SVC_ID\",\"kind\":\"prayer\"}")
PRES_ID=$(echo "$PRES" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
NSLIDES=$(echo "$PRES" | python -c 'import sys,json; print(len(json.load(sys.stdin)["data"]["slides"]))')
pass "presentation built with $NSLIDES slides"

BLOCK=$(curl -s -X POST "$BASE/presentations/$PRES_ID/exports" -H "$AUTH" -H 'Content-Type: application/json' -d '{"format":"pdf"}')
ERR=$(echo "$BLOCK" | python -c 'import sys,json; d=json.load(sys.stdin); print(d.get("error_code",""))')
[ "$ERR" = "SUMMARY_NOT_APPROVED" ] && pass "export blocked: SUMMARY_NOT_APPROVED" || fail "expected SUMMARY_NOT_APPROVED, got '$ERR'"

step "15. Approve every summary"
echo "$SUM" | python -c 'import sys,json; [print(s["id"]) for s in json.load(sys.stdin)["data"]]' | while read -r SID; do
  curl -sf -X PATCH "$BASE/ai/summaries/$SID" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"status":"approved"}' >/dev/null
done
pass "all summaries approved"

step "16. Export PDF"
EXPORT=$(curl -sf -X POST "$BASE/presentations/$PRES_ID/exports" -H "$AUTH" -H 'Content-Type: application/json' -d '{"format":"pdf"}')
EXPORT_ID=$(echo "$EXPORT" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
pass "export job $EXPORT_ID queued"

STATUS="queued"
for _ in $(seq 1 40); do
  sleep 1
  JOB=$(curl -sf "$BASE/presentations/$PRES_ID/exports/$EXPORT_ID" -H "$AUTH")
  STATUS=$(echo "$JOB" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["status"])')
  [ "$STATUS" = "done" ] && break
  [ "$STATUS" = "failed" ] && fail "export failed"
done
[ "$STATUS" = "done" ] && pass "PDF export completed" || fail "export timed out (status=$STATUS)"

URL=$(echo "$JOB" | python -c 'import sys,json; print(json.load(sys.stdin)["data"].get("download_url") or "")')
[ -n "$URL" ] && pass "download url issued" || fail "no download url"

step "17. Download the PDF and check the magic bytes"
curl -sf "$URL" -H "$AUTH" -o /tmp/legacy-export.pdf
HEAD=$(head -c 5 /tmp/legacy-export.pdf)
SIZE=$(wc -c < /tmp/legacy-export.pdf)
[ "$HEAD" = "%PDF-" ] && pass "valid PDF ($SIZE bytes)" || fail "not a PDF (got '$HEAD')"

step "18. PNG export (zip of slides)"
EXPORT2=$(curl -sf -X POST "$BASE/presentations/$PRES_ID/exports" -H "$AUTH" -H 'Content-Type: application/json' -d '{"format":"png"}')
EXPORT2_ID=$(echo "$EXPORT2" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
for _ in $(seq 1 40); do
  sleep 1
  JOB2=$(curl -sf "$BASE/presentations/$PRES_ID/exports/$EXPORT2_ID" -H "$AUTH")
  S2=$(echo "$JOB2" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["status"])')
  [ "$S2" = "done" ] && break
  [ "$S2" = "failed" ] && fail "png export failed"
done
URL2=$(echo "$JOB2" | python -c 'import sys,json; print(json.load(sys.stdin)["data"].get("download_url") or "")')
curl -sf "$URL2" -H "$AUTH" -o /tmp/legacy-export.zip
unzip -l /tmp/legacy-export.zip >/dev/null 2>&1 && pass "valid ZIP of PNGs" || fail "invalid zip"

step "19. Dashboard"
DASH=$(curl -sf "$BASE/dashboard/summary" -H "$AUTH")
echo "$DASH" | python -c '
import sys,json
d=json.load(sys.stdin)["data"]
assert d["next_service"], "no next service"
assert d["prayer_this_week"] >= 4, "prayer count wrong"
assert d["deadline_status"]["exported"] is True, "exported flag not set"
print("  PASS  dashboard: %d prayer this week, urgent=%d, exported=%s"
      % (d["prayer_this_week"], d["urgent_count"], d["deadline_status"]["exported"]))
'

step "20. Booth roster"
SHIFT=$(curl -sf -X POST "$BASE/services/$SVC_ID/booth-shifts" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"label\":\"Pre-service\",\"starts_at\":\"${SVC_DATE}T16:00:00+07:00\",\"ends_at\":\"${SVC_DATE}T17:00:00+07:00\",\"capacity\":2}")
SHIFT_ID=$(echo "$SHIFT" | python -c 'import sys,json; print(json.load(sys.stdin)["data"]["id"])')
USERS=$(curl -sf "$BASE/users" -H "$AUTH")
U1=$(echo "$USERS" | python -c 'import sys,json; d=json.load(sys.stdin)["data"]; print([u["id"] for u in d if u["role"]=="intercessor"][0])')
U2=$(echo "$USERS" | python -c 'import sys,json; d=json.load(sys.stdin)["data"]; print([u["id"] for u in d if u["role"]=="intercessor"][1])')
curl -sf -X POST "$BASE/booth-shifts/$SHIFT_ID/assignments" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"user_id\":\"$U1\"}" >/dev/null
curl -sf -X POST "$BASE/booth-shifts/$SHIFT_ID/assignments" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"user_id\":\"$U2\"}" >/dev/null
ROSTER=$(curl -sf "$BASE/services/$SVC_ID/booth-shifts" -H "$AUTH")
echo "$ROSTER" | python -c '
import sys,json
d=json.load(sys.stdin)["data"]
assert d[0]["covered"] is True, "shift not covered"
assert len(d[0]["assignees"])==2, "expected 2 assignees"
print("  PASS  roster: 2 intercessors assigned, shift covered")
'

step "21. Overbooking is rejected"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/booth-shifts/$SHIFT_ID/assignments" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"user_id\":\"$U1\"}")
[ "$CODE" = "422" ] || [ "$CODE" = "409" ] && pass "capacity enforced ($CODE)" || fail "expected 422/409, got $CODE"

step "22. Soft delete keeps the record out of lists"
curl -sf -X DELETE "$BASE/prayer-requests/$P4_ID" -H "$AUTH" >/dev/null
COUNT=$(curl -sf "$BASE/prayer-requests?q=peace" -H "$AUTH" | python -c 'import sys,json; print(json.load(sys.stdin)["meta"]["total"])')
[ "$COUNT" = "0" ] && pass "soft-deleted record hidden from search" || fail "soft delete failed"

step "23. Rate limit headers present"
curl -sf -D /tmp/h.txt -o /dev/null "$BASE/prayer-requests" -H "$AUTH"
grep -qi "x-ratelimit-limit" /tmp/h.txt && pass "X-RateLimit-Limit header present" || fail "missing rate limit headers"

echo
echo "================================================================"
echo " ALL SMOKE TESTS PASSED"
echo "================================================================"
