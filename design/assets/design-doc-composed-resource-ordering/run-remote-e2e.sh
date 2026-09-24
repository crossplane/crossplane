#!/usr/bin/env bash
# Run the composed resource ordering end-to-end tests on the GCE VM.
#
# Phase 1 is Crossplane's own ordering suite, against purpose-built fixtures.
# Phase 2 is Modelplane's e2e, which is the same feature under a real platform
# whose compositions were moved off Usages onto declared ordering.
#
# The VM is started if it's stopped, both repos are updated from the forks,
# and Crossplane is built from the branch. Phase 2 needs that build: the CLI
# installs Crossplane from charts.crossplane.io by version, so a branch build
# can't come in through it, and ordering is alpha and off unless the flag is
# passed.
#
# Usage:
#   ./run-remote-e2e.sh              # both phases
#   ./run-remote-e2e.sh phase1       # just the ordering suite (~7 min)
#   ./run-remote-e2e.sh phase2       # just Modelplane (~15 min)
#   ./run-remote-e2e.sh both --stop  # and stop the VM when done
#
# Everything runs detached on the VM, so losing the connection doesn't kill a
# run. Re-running this script tails the log of whatever is already going.
set -euo pipefail

INSTANCE="${INSTANCE:-borrelli-modelplane}"
PROJECT="${PROJECT:-crossplane-playground}"
ZONE="${ZONE:-us-central1-f}"

# Native Nix on the VM, not ./nix.sh: the wrapper runs Docker-in-Docker, so it
# would put the kind clusters somewhere kubectl can't reach and drop the
# CROSSPLANE_IMAGE and CROSSPLANE_ARGS the run needs.
#
# The ssh environment carries __ETC_PROFILE_NIX_SOURCED=1, which makes
# /etc/profile.d/nix.sh return early without extending PATH, so nix looks
# missing even under bash -lc. Hence the explicit PATH.
# shellcheck disable=SC2016  # $PATH must expand on the VM, not here.
NIX_PATH_PREFIX='export PATH=/nix/var/nix/profiles/default/bin:$PATH'

phase="${1:-both}"
stop_after=""
for arg in "$@"; do
	[ "${arg}" = "--stop" ] && stop_after=yes
done

case "${phase}" in
phase1 | phase2 | both | --stop) ;;
*)
	echo "usage: $0 [phase1|phase2|both] [--stop]" >&2
	exit 2
	;;
esac
[ "${phase}" = "--stop" ] && phase=both

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }

run() { gcloud compute ssh "${INSTANCE}" --project="${PROJECT}" --zone="${ZONE}" --quiet --command="$1"; }

# --- Make sure the VM is up ------------------------------------------------

state="$(gcloud compute instances describe "${INSTANCE}" --project="${PROJECT}" --zone="${ZONE}" --format='value(status)')"
if [ "${state}" != "RUNNING" ]; then
	log "starting ${INSTANCE} (was ${state})"
	gcloud compute instances start "${INSTANCE}" --project="${PROJECT}" --zone="${ZONE}" --quiet
fi

# sshd isn't up the moment the instance is, and a fresh start means a new host
# key, so the first attempt often fails. Retry rather than reporting that as a
# broken VM.
log "waiting for ssh"
for _ in $(seq 1 30); do
	if run 'true' >/dev/null 2>&1; then break; fi
	sleep 10
done
run 'true' >/dev/null

# --- Update both checkouts -------------------------------------------------

log "updating the checkouts"
# shellcheck disable=SC2016  # the command substitutions run on the VM.
run 'cd ~/code/crossplane && git pull -q && echo "crossplane $(git log --oneline -1)"
cd ~/code/modelplane && git pull -q && echo "modelplane $(git log --oneline -1)"'

# A leftover cluster from an interrupted run starves the next one: three kind
# clusters on one box exhausts inotify and kubeadm times out waiting for the
# kubelet, which reads as a broken build rather than a busy machine.
log "clearing leftover clusters"
# shellcheck disable=SC2016  # the command substitution runs on the VM.
run 'docker ps -aq | xargs -r docker rm -f >/dev/null 2>&1; docker network prune -f >/dev/null 2>&1; echo "containers: $(docker ps -aq | wc -l)"'

# --- Phase 1: Crossplane's ordering suite ----------------------------------

if [ "${phase}" = "phase1" ] || [ "${phase}" = "both" ]; then
	log "phase 1: Crossplane's ordering suite"
	run "${NIX_PATH_PREFIX}
cd ~/code/crossplane && setsid nohup nix run .#e2e -- --test-suite=composed-resource-ordering > ~/phase1.log 2>&1 < /dev/null & disown
echo launched"

	run "until grep -qE '^(DONE|FAIL|ok )' ~/phase1.log 2>/dev/null; do sleep 20; done
grep -E '^(--- PASS|--- FAIL)' ~/phase1.log
tail -2 ~/phase1.log"
fi

# --- Phase 2: Modelplane, against the branch build -------------------------

if [ "${phase}" = "phase2" ] || [ "${phase}" = "both" ]; then
	log "phase 2: building Crossplane from the branch"
	image="$(run "${NIX_PATH_PREFIX}
cd ~/code/crossplane && nix run .#stream-image 2>/dev/null | docker load 2>&1 | sed -n 's/^Loaded image: //p'" | tr -d '\r' | tail -1)"

	[ -n "${image}" ] || {
		echo "could not build the Crossplane image" >&2
		exit 1
	}

	log "phase 2: Modelplane e2e against ${image}"
	run "${NIX_PATH_PREFIX}
cd ~/code/modelplane && CROSSPLANE_IMAGE='${image}' CROSSPLANE_ARGS=--enable-composed-resource-ordering \
  setsid nohup nix run .#e2e -- --verify > ~/phase2.log 2>&1 < /dev/null & disown
echo launched"

	run "until grep -qE 'End to end OK|ERROR|error:' ~/phase2.log 2>/dev/null; do sleep 30; done
tail -4 ~/phase2.log"

	# A green verify says the stack converged, not that it converged in
	# order. xpgraph reads the edges and each resource's state off the XR,
	# which is the difference.
	log "the graph, as Crossplane ordered it"
	# ~/bin, not /tmp: a stopped VM clears /tmp, and rebuilding takes longer
	# than the rest of this step.
	run "${NIX_PATH_PREFIX}
mkdir -p ~/bin && cd ~/code/crossplane && nix develop -c go build -o ~/bin/xpgraph ./cmd/xpgraph 2>/dev/null
~/bin/xpgraph inferencegateway/default --context kind-modelplane-e2e-local --color never 2>&1 | head -20" || true
fi

# --- Tidy up ---------------------------------------------------------------

if [ -n "${stop_after}" ]; then
	log "tearing down the clusters and stopping ${INSTANCE}"
	run 'docker ps -aq | xargs -r docker rm -f >/dev/null 2>&1; true'
	gcloud compute instances stop "${INSTANCE}" --project="${PROJECT}" --zone="${ZONE}" --quiet
else
	log "done. The VM is still running - stop it with:"
	echo "  gcloud compute instances stop ${INSTANCE} --project=${PROJECT} --zone=${ZONE}"
fi
