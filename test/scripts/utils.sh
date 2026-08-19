#!/bin/bash

KUBECTL_RETRY=5
KUBECTL_RETRY_DELAY=10

# kubectl_retry calls `kubectl` with the passed arguments. In case of a
# failure, the `kubectl` command will be retried for `KUBECTL_RETRY` times,
# with a delay of `KUBECTL_RETRY_DELAY` between them.
#
# Upon creation failures, `AlreadyExists` and `Warning` are ignored, making
# sure the create succeeds in case some objects were created successfully in a
# previous try.
#
# Upon deletion failures, the same applies as for creation, except that
# NotFound is ignored.
#
# Logs from `kubectl` are passed on to stdout, so that a calling function can
# capture it. During the function, logs are written to stderr as to not
# interfere with the log parsing of the calling function.
kubectl_retry() {
    local retries=0 action="${1}" ret=0 stdout stderr
    shift

    # temporary files for kubectl output
    stdout=$(mktemp kubectl-stdout.XXXXXXXX)
    stderr=$(mktemp kubectl-stderr.XXXXXXXX)

    while ! ( kubectl "${action}" "${@}" 2>"${stderr}" 1>>"${stdout}" )
    do
        # write logs to stderr and empty stderr (only)
        cat "${stdout}" > /dev/stderr
        cat "${stderr}" > /dev/stderr
        echo "$(date): 'kubectl_retry ${action} ${*}' try #${retries} failed, checking errors" > /dev/stderr

        # in case of a failure when running "create", ignore errors with "AlreadyExists"
        if [ "${action}" == 'create' ]
        then
            # count lines in stderr that do not have "AlreadyExists" or "Warning"
            ret=$(grep -cvw -e 'AlreadyExists' -e '^Warning:' "${stderr}" || true)
            if [ "${ret}" -eq 0 ]
            then
                # Success! stderr is empty after removing all "AlreadyExists" lines.
                echo "$(date): 'kubectl_retry ${action} ${*}' succeeded without unknown errors" > /dev/stderr
                break
            fi
        fi

        # in case of a failure when running "delete", ignore errors with "NotFound"
        if [ "${action}" == 'delete' ]
        then
            # count lines in stderr that do not have "NotFound" or "Warning"
            ret=$(grep -cvw -e 'NotFound' -e '^Warning:' "${stderr}" || true)
            if [ "${ret}" -eq 0 ]
            then
                # Success! stderr is empty after removing all "NotFound" lines.
                echo "$(date): 'kubectl_retry ${action} ${*}' succeeded without unknown errors" > /dev/stderr
                break
            fi
        fi

        retries=$((retries+1))
        if [ ${retries} -eq ${KUBECTL_RETRY} ]
        then
            echo "$(date): 'kubectl_retry ${action} ${*}' failed, no more retries left (${retries}/${KUBECTL_RETRY})" > /dev/stderr
            ret=1
            break
        fi

        # empty stderr for the next loop
        true > "${stderr}"
        echo "$(date): 'kubectl_retry ${action} ${*}' failed (${retries}/${KUBECTL_RETRY}), will retry in ${KUBECTL_RETRY_DELAY} seconds" > /dev/stderr

        sleep ${KUBECTL_RETRY_DELAY}

        # reset ret so that a next working kubectl does not cause a non-zero
        # return of the function
        ret=0
    done

    echo "$(date): 'kubectl_retry ${action} ${*}' done (ret=${ret})" > /dev/stderr

    # write output so that calling functions can consume it
    cat "${stdout}" > /dev/stdout
    cat "${stderr}" > /dev/stderr

    rm -f "${stdout}" "${stderr}"

    return ${ret}
}

OPERATOR_NAMESPACE=${OPERATOR_NAMESPACE:-"ceph-csi-operator-system"}
DRIVER_NAMESPACE=${DRIVER_NAMESPACE:-"csi-driver"}
STABILITY_WAIT=${STABILITY_WAIT:-15}
MAX_RESTART_COUNT=${MAX_RESTART_COUNT:-2}

dump_namespace_info() {
    local ns="$1"

    echo "--- Pods in ${ns} ---"
    kubectl get pods -n "${ns}" -o wide 2>/dev/null || true

    echo "--- Pod details in ${ns} ---"
    kubectl get pods -n "${ns}" -o custom-columns=\
'NAME:.metadata.name,STATUS:.status.phase,READY:.status.conditions[?(@.type=="Ready")].status,RESTARTS:.status.containerStatuses[*].restartCount,AGE:.metadata.creationTimestamp' \
        2>/dev/null || true

    echo "--- Not-ready container reasons in ${ns} ---"
    kubectl get pods -n "${ns}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{range .status.containerStatuses[*]}  {.name}: ready={.ready} restarts={.restartCount}{"\n"}{end}{range .status.conditions[*]}  condition {.type}={.status} reason={.reason} message={.message}{"\n"}{end}{"\n"}{end}' \
        2>/dev/null || true

    echo "--- Events in ${ns} (last 20) ---"
    kubectl get events -n "${ns}" --sort-by='.lastTimestamp' 2>/dev/null | tail -20 || true

    echo "--- Deployments/DaemonSets in ${ns} ---"
    kubectl get deployment,daemonset,replicaset -n "${ns}" 2>/dev/null || true

    echo "--- Pod logs (last 30 lines each) in ${ns} ---"
    for pod in $(kubectl get pods -n "${ns}" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
        for container in $(kubectl get pod "${pod}" -n "${ns}" -o jsonpath='{.spec.containers[*].name}' 2>/dev/null); do
            echo ">>> ${pod}/${container}:"
            kubectl logs "${pod}" -n "${ns}" -c "${container}" --tail=30 2>/dev/null || echo "(no logs)"
        done
    done
}

check_operator_health() {
    echo "Checking operator health in ${OPERATOR_NAMESPACE}"

    local deploy_name
    deploy_name=$(kubectl -n "${OPERATOR_NAMESPACE}" get deployment -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [[ -z "$deploy_name" ]]; then
        echo "ERROR: No operator deployment found in ${OPERATOR_NAMESPACE}"
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi
    echo "Found operator deployment: ${deploy_name}"

    if ! kubectl rollout status "deployment/${deploy_name}" -n "${OPERATOR_NAMESPACE}" --timeout=300s; then
        echo "ERROR: Operator rollout did not complete"
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi

    echo "Waiting for operator pod to be Running..."
    local running_count=0
    for _ in {1..60}; do
        running_count=$(kubectl -n "${OPERATOR_NAMESPACE}" get pods \
            --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
        if [[ "$running_count" -gt 0 ]]; then
            break
        fi
        sleep 2
    done
    if [[ "$running_count" -eq 0 ]]; then
        echo "ERROR: No operator pod running after 2 minutes"
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi

    echo "Operator pod is Running, waiting ${STABILITY_WAIT}s to verify stability..."
    sleep "${STABILITY_WAIT}"

    running_count=$(kubectl -n "${OPERATOR_NAMESPACE}" get pods --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    if [[ "$running_count" -eq 0 ]]; then
        echo "ERROR: Operator pod stopped running after stability wait"
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi

    local restart_count
    restart_count=$(kubectl -n "${OPERATOR_NAMESPACE}" get pods --field-selector=status.phase=Running \
        -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
    if [[ "$restart_count" -gt "$MAX_RESTART_COUNT" ]]; then
        echo "ERROR: Operator has too many restarts (${restart_count} > ${MAX_RESTART_COUNT})"
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi

    echo "Operator is healthy (restarts: ${restart_count})"
}

check_driver_health() {
    local driver_type="${1:-${DRIVER_TYPE:-rbd}}"
    echo "Checking driver health for ${driver_type} in ${DRIVER_NAMESPACE}"

    local expected_pod_count=2
    if [ "$driver_type" = "rbd" ]; then
        expected_pod_count=3
    fi

    local running_pods=0
    for _ in {1..180}; do
        running_pods=$(kubectl get pods -n "${DRIVER_NAMESPACE}" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
        if [ "$running_pods" -eq "$expected_pod_count" ]; then
            echo "All ${expected_pod_count} CSI driver pods are running"
            break
        fi
        sleep 1
    done

    if [ "$running_pods" -ne "$expected_pod_count" ]; then
        echo "ERROR: Timeout waiting for driver pods (expected ${expected_pod_count}, got ${running_pods})"
        dump_namespace_info "${DRIVER_NAMESPACE}"
        echo "=== Operator namespace ==="
        dump_namespace_info "${OPERATOR_NAMESPACE}"
        return 1
    fi

    echo "Waiting ${STABILITY_WAIT}s to verify driver pod stability..."
    sleep "${STABILITY_WAIT}"

    running_pods=$(kubectl get pods -n "${DRIVER_NAMESPACE}" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
    if [ "$running_pods" -ne "$expected_pod_count" ]; then
        echo "ERROR: Driver pods stopped running after stability wait (${running_pods}/${expected_pod_count})"
        dump_namespace_info "${DRIVER_NAMESPACE}"
        return 1
    fi

    local max_restarts=0
    while IFS= read -r count; do
        if [ "$count" -gt "$max_restarts" ]; then
            max_restarts=$count
        fi
    done < <(kubectl get pods -n "${DRIVER_NAMESPACE}" -o jsonpath='{range .items[*]}{range .status.containerStatuses[*]}{.restartCount}{"\n"}{end}{end}' 2>/dev/null || echo "0")

    if [ "$max_restarts" -gt "$MAX_RESTART_COUNT" ]; then
        echo "ERROR: Driver pods have too many restarts (max: ${max_restarts} > ${MAX_RESTART_COUNT})"
        dump_namespace_info "${DRIVER_NAMESPACE}"
        return 1
    fi

    echo "Driver pods are healthy (max restarts: ${max_restarts})"
}

