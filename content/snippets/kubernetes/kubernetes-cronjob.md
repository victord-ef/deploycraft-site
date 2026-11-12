---
title: "Kubernetes CronJob — Scheduled Jobs with Concurrency and History Controls"
date: 2026-11-12
description: "CronJob manifest with concurrency policy to prevent overlapping runs, history limits to avoid stale pod accumulation, a deadline for late-starting jobs, and a security context matching production workloads."
lang: "Kubernetes"
tags: ["kubernetes", "cronjob", "job", "scheduling", "batch", "security-context"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

For any recurring batch task — database cleanup, report generation, cache warming, certificate rotation checks, or snapshot triggers. The defaults for CronJob leave history accumulating indefinitely and allow multiple instances to run simultaneously, which breaks most batch workloads.

## Without it

```yaml
# Minimal CronJob — dangerous defaults
apiVersion: batch/v1
kind: CronJob
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: job
              image: my-job:latest
          restartPolicy: OnFailure
# No concurrency control — two runs overlap if the first takes > 1 hour
# No history limit — failed and succeeded pods accumulate indefinitely
# No deadline — a job waiting for a node may run hours late
# image:latest — non-deterministic, pulls whatever is current
```

## Snippet

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: db-cleanup
  namespace: jobs
spec:
  # Standard cron syntax — this runs at 02:00 UTC daily
  schedule: "0 2 * * *"
  timeZone: "UTC"            # explicit timezone (requires k8s >= 1.27)

  # Forbid: skip the new run if the previous one is still running
  # Replace: cancel the running job and start a new one
  # Allow: run both simultaneously (use only for truly independent jobs)
  concurrencyPolicy: Forbid

  # How many seconds past the scheduled time the job may still start
  # A pod stuck in Pending past this deadline is marked as missed
  startingDeadlineSeconds: 300

  # How many finished/failed job records to keep (for kubectl get jobs / logs)
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 5

  jobTemplate:
    spec:
      # Retry at most 2 times before marking the job as failed
      backoffLimit: 2

      # Kill the job if it runs longer than 1 hour (prevents zombie jobs)
      activeDeadlineSeconds: 3600

      template:
        metadata:
          labels:
            app: db-cleanup
            job-type: maintenance
        spec:
          restartPolicy: OnFailure    # Never for jobs that must not retry per-pod

          serviceAccountName: db-cleanup-sa
          automountServiceAccountToken: false

          securityContext:
            runAsNonRoot: true
            runAsUser: 10001
            runAsGroup: 10001
            fsGroup: 10001
            seccompProfile:
              type: RuntimeDefault

          containers:
            - name: cleanup
              image: my-job:1.4.2       # pinned tag — never use latest in production
              imagePullPolicy: IfNotPresent

              command: ["/usr/local/bin/cleanup"]
              args:
                - "--older-than=30d"
                - "--dry-run=false"

              env:
                - name: DATABASE_URL
                  valueFrom:
                    secretKeyRef:
                      name: db-credentials
                      key: url
                - name: LOG_LEVEL
                  value: "info"

              resources:
                requests:
                  cpu: 100m
                  memory: 128Mi
                limits:
                  cpu: 500m
                  memory: 256Mi

              securityContext:
                allowPrivilegeEscalation: false
                readOnlyRootFilesystem: true
                capabilities:
                  drop: ["ALL"]

              volumeMounts:
                - name: tmp
                  mountPath: /tmp   # writable scratch space for a read-only rootfs

          volumes:
            - name: tmp
              emptyDir: {}
```

### One-off Job triggered manually (same template as CronJob)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: db-cleanup-manual-20261112
  namespace: jobs
spec:
  backoffLimit: 1
  activeDeadlineSeconds: 3600
  template:
    spec:
      restartPolicy: Never
      serviceAccountName: db-cleanup-sa
      containers:
        - name: cleanup
          image: my-job:1.4.2
          command: ["/usr/local/bin/cleanup"]
          args: ["--older-than=30d", "--dry-run=true"]    # dry-run for manual runs
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
```

### Trigger a CronJob immediately without waiting for the schedule

```bash
kubectl create job --from=cronjob/db-cleanup \
  db-cleanup-adhoc-$(date +%s) \
  -n jobs
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `concurrencyPolicy: Forbid` | Most batch jobs are not idempotent — running two cleanup jobs simultaneously deletes the same rows twice or corrupts aggregation results. `Forbid` skips the new run rather than causing data issues. |
| `startingDeadlineSeconds: 300` | A job waiting for a node may start 20 minutes late. Without a deadline, it runs at whatever time it finally starts — causing a cleanup job scheduled for 02:00 to run at 03:00, overlapping with business hours. |
| `activeDeadlineSeconds: 3600` | A job stuck in an infinite loop or waiting on an unavailable dependency runs forever and blocks future runs under `Forbid` policy. A deadline kills it and allows the next scheduled run to proceed. |
| `restartPolicy: OnFailure` over `Never` | `OnFailure` restarts the container within the same pod on failure, up to `backoffLimit`. `Never` creates a new pod per failure, which produces many pods and exhausts the history limit quickly. Use `Never` only when each retry must start completely fresh. |
| `successfulJobsHistoryLimit: 3` | Without history limits, every completed Job and its pod remain in the namespace indefinitely. At hourly frequency, that is 720 stale pods per month. Three successful records are enough to verify recent runs. |
| `failedJobsHistoryLimit: 5` | Keeps more failed records than successful ones — you need failed pod logs to diagnose recurring failures. Five gives a window of recent failures without keeping every failed run since deployment. |
| `timeZone: "UTC"` explicit | The default timezone for CronJob schedules is the kube-controller-manager's timezone, which varies by cluster configuration. Explicit `UTC` removes this ambiguity entirely. |

## Verify it worked

```bash
# View scheduled jobs and their last run time
kubectl get cronjob db-cleanup -n jobs

# List the Job records created by recent runs
kubectl get jobs -n jobs -l app=db-cleanup

# View logs from the most recent run
kubectl logs -n jobs \
  $(kubectl get pods -n jobs -l app=db-cleanup \
    --sort-by=.metadata.creationTimestamp \
    -o jsonpath='{.items[-1].metadata.name}')

# Trigger a manual run immediately
kubectl create job --from=cronjob/db-cleanup \
  db-cleanup-test-$(date +%s) -n jobs

# Watch the job complete
kubectl get job -n jobs -w
```
