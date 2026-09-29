#!/usr/bin/env bash
# Watch the ordering graph with crossplane-graph while resources are deleted and
# recreated around it.
#
# Two halves, meant to run side by side:
#
#   ./crossplane-graph-demo.sh watch [demo|modelplane]   # the graph, redrawn every second
#   ./crossplane-graph-demo.sh run   [demo|modelplane]   # the beats, one per Enter
#
# Inside tmux, `run` opens the watch in a pane of its own.
#
# demo (the default) is the XNetwork from ./up.sh: ten NopResources, seconds
# apart. Its beats:
#   1. Compose the XR and let it converge.
#   2. Drop database and backup from the pipeline while the XR stays up.
#      backup is deleted first, and database waits for it.
#   3. Put them back. database is created first, and backup waits for it.
#   4. Delete the XR. Teardown runs leaves first, without the pipeline.
#   5. Recreate the XR. Creation runs in waves, roots first.
#
# modelplane is the ServingStack from Modelplane's e2e (`nix run .#e2e`),
# composed by InferenceCluster/local. Helm releases rather than NopResources,
# so each wave takes minutes rather than seconds. Its beats:
#   1. Show the converged stack.
#   2. Delete one composed resource directly. Crossplane recreates it, and
#      leaves its dependents alone: ordering governs Crossplane's own creates
#      and deletes, not anyone else's.
#   3. Delete the ServingStack. It tears down leaves first. Once it has gone,
#      the InferenceCluster composes it again, and it comes back in waves.
#
# Settings:
#   CONTEXT   kubeconfig context (default kind-xp-ordering-demo for demo,
#             kind-modelplane-e2e-local for modelplane)
#   GRAPH   path to a crossplane-graph binary (default: on PATH, else installed)
#   RESOURCE  modelplane only: the composition resource name beat 2 deletes.
#             The default, a ResourceQuota, comes back in seconds.
#             RESOURCE=cert-manager deletes a Helm release instead, which
#             takes a few minutes to reinstall.
#   EDGES=1   show each resource's dependencies, not only the waves
#   AUTO=N    advance every N seconds instead of waiting for Enter
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "${here}/../../../.." && pwd)"

mode="${1:-}"
target="${2:-demo}"

case "${target}" in
demo)
	context="${CONTEXT:-kind-xp-ordering-demo}"
	ns=default
	xr=xnetworks.demo.crossplane.io/demo
	;;
modelplane)
	context="${CONTEXT:-kind-modelplane-e2e-local}"
	ns=modelplane-system
	xrtype=servingstacks.infrastructure.modelplane.ai
	;;
*)
	echo "unknown target ${target}; want demo or modelplane" >&2
	exit 2
	;;
esac

kc() { kubectl --context "${context}" -n "${ns}" "$@"; }

# find_graph sets GRAPH, installing crossplane-graph if there is none.
find_graph() {
	if [ -n "${GRAPH:-}" ]; then
		return
	fi
	if command -v crossplane-graph >/dev/null; then
		GRAPH="$(command -v crossplane-graph)"
		return
	fi

	GRAPH="${XDG_CACHE_HOME:-${HOME}/.cache}/crossplane-graph/crossplane-graph"
	if [ -x "${GRAPH}" ]; then
		return
	fi

	echo "==> installing crossplane-graph into $(dirname "${GRAPH}")" >&2
	mkdir -p "$(dirname "${GRAPH}")"
	local pkg=github.com/stevendborrelli/crossplane-graph@latest
	if command -v go >/dev/null; then
		GOBIN="$(dirname "${GRAPH}")" go install "${pkg}"
	else
		# No Go here; the checkout's Nix shell has one.
		(cd "${repo}" && GOBIN="$(dirname "${GRAPH}")" nix develop -c go install "${pkg}")
	fi
}

# find_servingstack sets xr to the ServingStack's type/name.
#
# The InferenceCluster names it, and recreates it under the same name, so
# looking it up once is enough - which matters, because beat 3 deletes it and
# the watch has to keep pointing at it while it is gone.
find_servingstack() {
	local name
	name="$(kc get "${xrtype}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
	if [ -z "${name}" ]; then
		echo "no ServingStack in ${ns} on ${context}; is Modelplane's e2e up?" >&2
		exit 1
	fi
	xr="${xrtype}/${name}"
}

watch_graph() {
	find_graph
	[ "${target}" = modelplane ] && find_servingstack

	local flags=(-n "${ns}" --context "${context}" --color always)
	[ -n "${EDGES:-}" ] && flags+=(--edges)

	# The XR is absent before the first beat and between teardown and
	# recreation. Say so rather than filling the pane with an error.
	exec watch -c -t -n1 "'${GRAPH}' ${xr} ${flags[*]} 2>/dev/null || echo '(no XR)'"
}

open_watch() {
	if [ -n "${TMUX:-}" ]; then
		tmux split-window -h -d "CONTEXT='${context}' GRAPH='${GRAPH}' EDGES='${EDGES:-}' '${here}/crossplane-graph-demo.sh' watch ${target}"
	else
		echo "Run '${here}/crossplane-graph-demo.sh watch ${target}' in another terminal to see the graph."
	fi
}

beat() {
	printf '\n\033[1;34m==> %s\033[0m\n' "$1"
	shift
	for line in "$@"; do
		printf '    %s\n' "${line}"
	done
	if [ -n "${AUTO:-}" ]; then
		sleep "${AUTO}"
	else
		read -r -p "    [Enter] " _
	fi
}

# progress rewrites one line with how long a wait has taken, and why.
progress() {
	printf '\r\033[K    %4ds  %s' "$1" "${2:0:110}"
}

# wait_ready waits for the XR to exist and report Ready, saying what it is
# waiting on while it does - a Helm release can take minutes, and a silent
# wait that long reads as a hang.
wait_ready() {
	local timeout="$1" start="${SECONDS}" status msg
	while :; do
		status="$(kc get "${xr}" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
		[ "${status}" = True ] && break
		if ((SECONDS - start >= timeout)); then
			printf '\n!! %s not Ready after %ds\n' "${xr}" "${timeout}" >&2
			exit 1
		fi
		msg="$(kc get "${xr}" -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}' 2>/dev/null || true)"
		progress "$((SECONDS - start))" "${msg:-waiting for the XR}"
		sleep 2
	done
	printf '\r\033[K    ready after %ds\n' "$((SECONDS - start))"
}

# wait_deleted waits for the XR to go, counting what teardown still holds back.
wait_deleted() {
	local timeout="$1" start="${SECONDS}" pending
	while kc get "${xr}" >/dev/null 2>&1; do
		if ((SECONDS - start >= timeout)); then
			printf '\n!! %s still present after %ds\n' "${xr}" "${timeout}" >&2
			exit 1
		fi
		pending="$(kc get "${xr}" -o jsonpath='{.status.crossplane.pendingResources[*].resourceName}' 2>/dev/null | wc -w)"
		progress "$((SECONDS - start))" "tearing down, ${pending// /} held back"
		sleep 2
	done
	printf '\r\033[K    deleted after %ds\n' "$((SECONDS - start))"
}

# wait_gone waits until no NopResource has any of the given names.
wait_gone() {
	local pattern
	pattern="$(IFS='|'; echo "$*")"
	for _ in $(seq 1 180); do
		if ! kc get nopresources.nop.crossplane.io \
			-o jsonpath='{range .items[*]}{.metadata.annotations.crossplane\.io/composition-resource-name}{"\n"}{end}' 2>/dev/null |
			grep -qxE "${pattern}"; then
			echo "    gone: $*"
			return
		fi
		sleep 1
	done
	echo "!! still present after 3m: $*" >&2
	exit 1
}

run_demo() {
	kc get xrd xnetworks.demo.crossplane.io >/dev/null 2>&1 || {
		echo "no XNetwork XRD on ${context}; run ./up.sh first" >&2
		exit 1
	}
	find_graph
	open_watch

	# Start from a clean XR, so every beat begins from the same graph.
	kc delete "${xr}" --ignore-not-found --wait >/dev/null
	kc apply -f "${here}/manifests/02-teardown.yaml" >/dev/null

	beat "1. Compose the XR" \
		"Ten resources, created a dependency level at a time." \
		"Resources that are still waiting appear under pending, with the reason."
	kc apply -f "${here}/manifests/xr.yaml" >/dev/null
	wait_ready 180

	beat "2. Drop database and backup from the pipeline" \
		"The XR stays up. backup is deleted first, and database is held back" \
		"until backup has gone: 'waiting for [backup] to be deleted'."
	kc apply -f "${here}/manifests/04-without-database.yaml" >/dev/null
	wait_gone database backup

	beat "3. Put them back" \
		"database is created first, and backup waits for it to be ready."
	kc apply -f "${here}/manifests/02-teardown.yaml" >/dev/null
	wait_ready 180

	beat "4. Delete the XR" \
		"No function runs. Crossplane rebuilds the graph from" \
		"spec.crossplane.resourceRefs and deletes leaves first, vpc last."
	kc delete "${xr}" --wait=false >/dev/null
	wait_deleted 180

	beat "5. Recreate the XR" \
		"Back in dependency order, roots first."
	kc apply -f "${here}/manifests/xr.yaml" >/dev/null
	wait_ready 180

	printf '\nDone. The XR is still up; ./down.sh removes the cluster.\n'
}

# composed prints "<kind>.<group> <name> <uid>" for the composed resource with
# the given composition resource name, or nothing if there is none.
composed() {
	local ref kind api name
	ref="$(kc get "${xr}" -o jsonpath="{range .spec.crossplane.resourceRefs[?(@.resourceName==\"$1\")]}{.kind} {.apiVersion} {.name}{end}")"
	[ -n "${ref}" ] || return 0
	read -r kind api name <<<"${ref}"
	# A core resource has no group, and kubectl wants the bare kind for it.
	if [[ "${api}" == */* ]]; then
		kind="${kind}.${api%/*}"
	fi
	printf '%s %s %s\n' "${kind}" "${name}" "$(kc get "${kind}" "${name}" -o jsonpath='{.metadata.uid}' 2>/dev/null)"
}

run_modelplane() {
	find_graph
	find_servingstack
	open_watch

	local resource="${RESOURCE:-dra-driver-critical-pods-quota}"
	local kind name uid
	read -r kind name uid <<<"$(composed "${resource}")"
	if [ -z "${uid:-}" ]; then
		echo "no composed resource named ${resource} on ${xr}. It has:" >&2
		kc get "${xr}" -o jsonpath='{range .spec.crossplane.resourceRefs[*]}{"  "}{.resourceName}{"\n"}{end}' >&2
		exit 1
	fi

	beat "1. The serving stack, converged" \
		"${xr#*/}, composed by InferenceCluster/local." \
		"Every resource is in a wave, and nothing is pending."
	wait_ready 900

	beat "2. Delete ${resource} directly" \
		"kubectl delete ${kind} ${name}" \
		"Crossplane recreates it once what it depends on is Ready, which it" \
		"already is. Its dependents stay put: ordering governs Crossplane's own" \
		"deletes, not anyone else's. Use a Usage to stop a delete like this."
	kc delete "${kind}" "${name}" --wait=false >/dev/null
	local start="${SECONDS}" now
	while :; do
		now="$(kc get "${kind}" "${name}" -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
		if [ -n "${now}" ] && [ "${now}" != "${uid}" ]; then
			break
		fi
		if ((SECONDS - start >= 600)); then
			printf '\n!! %s was not recreated within 600s\n' "${resource}" >&2
			exit 1
		fi
		progress "$((SECONDS - start))" "${now:+deleting}${now:-deleted, waiting for Crossplane to recreate it}"
		sleep 2
	done
	printf '\r\033[K    recreated after %ds\n' "$((SECONDS - start))"
	wait_ready 900

	beat "3. Delete the ServingStack" \
		"Its own teardown runs first, leaves first, ProviderConfigs last." \
		"Once it has gone, the InferenceCluster composes it again, and it" \
		"comes back a wave at a time. The pane shows (no XR) in between."
	kc delete "${xr}" --wait=false >/dev/null
	wait_deleted 900
	echo "    waiting for the InferenceCluster to recompose it"
	wait_ready 1200

	printf '\nDone. The stack is back up.\n'
}

case "${mode}" in
watch) watch_graph ;;
run)
	if [ "${target}" = modelplane ]; then
		run_modelplane
	else
		run_demo
	fi
	;;
*)
	echo "usage: $0 watch|run [demo|modelplane]" >&2
	exit 2
	;;
esac
