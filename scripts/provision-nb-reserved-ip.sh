#!/usr/bin/env bash
set -euo pipefail

# -----------------------------------------------------------------------------
# Provision a NodeBalancer fronted by a portable Reserved IP, then emit the
# `terraform import` command to bring it under Terraform management.
#
# WHY THIS EXISTS: assigning a Reserved IP to a NodeBalancer is supported by the
# Linode API (the `ipv4` field on NB create) but NOT by the Terraform provider —
# `linode_nodebalancer.ipv4` is computed-only (verified on provider 2.41.2 and
# 3.14.1). So we create the NB out-of-band with the reserved IP, then import it.
# Verified: after import, `terraform plan` reports "No changes" (no ipv4 drift).
# See ADR-010.
#
# The Reserved IP lives OUTSIDE Terraform state on purpose: it persists on the
# account across `terraform destroy` (the NB releases it back, unassigned), so
# PNC DNS target never changes. On a rebuild, re-run with the SAME reserved IP.
#
# Usage:
#   LINODE_TOKEN=<token> ./provision-nb-reserved-ip.sh <region> <nb-label> <tf-module-addr> [reserved-ip]
#
#   region          Linode region slug          e.g. us-ord
#   nb-label        MUST match the module NB label (local.prefix + "-nb")
#                                               e.g. tld-ord-nb
#   tf-module-addr  Terraform module address    e.g. module.ord
#   reserved-ip     (optional) reuse an existing reserved IP; if omitted, a new
#                   one is reserved and printed — RECORD IT for rebuilds.
#
# Example (both regions):
#   LINODE_TOKEN=$PNC_TOKEN ./provision-nb-reserved-ip.sh us-ord tld-ord-nb module.ord
#   LINODE_TOKEN=$PNC_TOKEN ./provision-nb-reserved-ip.sh us-iad tld-iad-nb module.iad
# -----------------------------------------------------------------------------

API="https://api.linode.com/v4"
: "${LINODE_TOKEN:?set LINODE_TOKEN to the target account API token}"

REGION="${1:?usage: <region> <nb-label> <tf-module-addr> [reserved-ip]}"
LABEL="${2:?usage: <region> <nb-label> <tf-module-addr> [reserved-ip]}"
TFMOD="${3:?usage: <region> <nb-label> <tf-module-addr> [reserved-ip]}"
RIP="${4:-}"

AUTH=(-H "Authorization: Bearer ${LINODE_TOKEN}" -H "Content-Type: application/json")

json_field() { python3 -c "import sys,json;d=json.load(sys.stdin);print(d['$1']) if '$1' in d else sys.exit('API error: '+json.dumps(d))"; }

if [ -z "$RIP" ]; then
  echo "==> reserving a new IPv4 in ${REGION}..."
  RIP=$(curl -s "${AUTH[@]}" -X POST "${API}/networking/ips" \
        -d "{\"type\":\"ipv4\",\"public\":true,\"reserved\":true,\"region\":\"${REGION}\"}" | json_field address)
  echo "    reserved: ${RIP}"
  echo "    >> RECORD THIS ADDRESS. Reuse it on every rebuild so PNC never re-points DNS."
else
  echo "==> reusing existing reserved IP: ${RIP}"
fi

echo "==> creating NodeBalancer '${LABEL}' in ${REGION} with ipv4=${RIP}..."
NBID=$(curl -s "${AUTH[@]}" -X POST "${API}/nodebalancers" \
       -d "{\"region\":\"${REGION}\",\"label\":\"${LABEL}\",\"ipv4\":\"${RIP}\"}" | json_field id)

# Verify the NB actually carries the reserved IP
GOT=$(curl -s "${AUTH[@]}" "${API}/nodebalancers/${NBID}" | json_field ipv4)
if [ "$GOT" != "$RIP" ]; then
  echo "ERROR: NB ${NBID} has ipv4 ${GOT}, expected ${RIP}" >&2
  exit 1
fi
echo "    created NB ${NBID}, ipv4 confirmed = ${RIP}"

cat <<EOF

Next: import the NB into Terraform, then apply (creates configs, nodes, firewall):

  cd terraform/environments/prod
  terraform import '${TFMOD}.linode_nodebalancer.data' ${NBID}
  terraform apply

To release the reserved IP later (permanent, stops charges):
  curl -H "Authorization: Bearer \$LINODE_TOKEN" -X DELETE \\
    ${API}/networking/reserved-ips/${RIP}
EOF
