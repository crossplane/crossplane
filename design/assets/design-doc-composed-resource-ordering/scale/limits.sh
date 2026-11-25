#!/usr/bin/env bash
# Raise (or restore) the resource limits and concurrency of a running
# Crossplane, for runs large enough that the defaults bind.
#
# The Helm chart caps Crossplane at 500m CPU and 1Gi of memory, and core
# reconciles at most --max-reconcile-rate XRs concurrently. Those are sensible
# defaults for a shared cluster and wrong for a scale measurement: past a few
# hundred composed resources the numbers describe the CPU limit rather than
# the ordering, and a run cannot be compared with a smaller one that never hit
# it.
#
# Raising them proves the point in one direction only. If a run gets faster
# when the limit goes up, the limit was binding. If it doesn't, the limit was
# not what paced it - which is the more interesting answer.
#
# Usage:
#   ./limits.sh big          # 4 CPU, 8Gi, --max-reconcile-rate=100
#   ./limits.sh huge         # 6 CPU, 16Gi, --max-reconcile-rate=200
#   ./limits.sh default      # back to the chart's 500m / 1Gi, rate 10
#   CPU=2 MEM=4Gi RATE=50 ./limits.sh custom
set -euo pipefail

profile="${1:-big}"

case "${profile}" in
big)
	cpu="${CPU:-4}"
	mem="${MEM:-8Gi}"
	rate="${RATE:-100}"
	;;
huge)
	cpu="${CPU:-6}"
	mem="${MEM:-16Gi}"
	rate="${RATE:-200}"
	;;
default)
	cpu="${CPU:-500m}"
	mem="${MEM:-1024Mi}"
	rate="${RATE:-10}"
	;;
custom)
	cpu="${CPU:?set CPU}"
	mem="${MEM:?set MEM}"
	rate="${RATE:?set RATE}"
	;;
*)
	echo "usage: $0 [big|huge|default|custom]" >&2
	exit 2
	;;
esac

echo "==> crossplane: cpu=${cpu} memory=${mem} max-reconcile-rate=${rate}"

# The args are replaced wholesale rather than appended to, so re-running with a
# different rate doesn't leave two --max-reconcile-rate flags behind.
args="$(kubectl -n crossplane-system get deploy crossplane \
	-o jsonpath='{.spec.template.spec.containers[0].args}' |
	python3 -c "
import json,sys
args=[a for a in json.load(sys.stdin) if not a.startswith('--max-reconcile-rate')]
args.append('--max-reconcile-rate=${rate}')
print(json.dumps(args))
")"

kubectl -n crossplane-system patch deploy crossplane --type=json -p "[
 {\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/args\",\"value\":${args}},
 {\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/resources\",\"value\":{
   \"limits\":{\"cpu\":\"${cpu}\",\"memory\":\"${mem}\"},
   \"requests\":{\"cpu\":\"100m\",\"memory\":\"256Mi\"}}}
]"
kubectl -n crossplane-system rollout status deploy/crossplane --timeout=5m

# provider-nop reconciles every composed resource, so it is the other half of
# the budget. Its deployment is managed by the package manager, which resets
# what it owns - the DeploymentRuntimeConfig is where a change sticks.
echo "==> provider-nop: cpu=${cpu} memory=${mem}"
kubectl patch deploymentruntimeconfig provider-nop-fast-poll --type=merge -p "{
 \"spec\":{\"deploymentTemplate\":{\"spec\":{\"template\":{\"spec\":{\"containers\":[{
   \"name\":\"package-runtime\",
   \"args\":[\"--poll=1s\",\"--max-reconcile-rate=${rate}\"],
   \"resources\":{\"limits\":{\"cpu\":\"${cpu}\",\"memory\":\"${mem}\"},
                  \"requests\":{\"cpu\":\"100m\",\"memory\":\"256Mi\"}}
 }]}}}}}
}"

# The package manager rolls the provider deployment itself once the runtime
# config changes; wait for whatever it produces.
sleep 5
for d in $(kubectl -n crossplane-system get deploy -l pkg.crossplane.io/provider=provider-nop -o name); do
	kubectl -n crossplane-system rollout status "${d}" --timeout=5m
done

echo
echo "crossplane:"
kubectl -n crossplane-system get deploy crossplane \
	-o jsonpath='{.spec.template.spec.containers[0].resources}{"\n"}'
kubectl -n crossplane-system get deploy crossplane \
	-o jsonpath='{.spec.template.spec.containers[0].args}{"\n"}'
