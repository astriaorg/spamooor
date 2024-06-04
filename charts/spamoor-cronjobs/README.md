# spamoor-cronjobs
You can define an array of jobs in values.yaml helm will take care of creating all of the CronJobs.

Credit: [helm-cronjobs](https://github.com/bambash/helm-cronjobs) used as scafolding for this chart.

## Getting started

`helm create -p <scaffolding_path> <new_chart_name>`

  ```
    helm create -p helm-cronjobs spamoor-cronjobs
  ```

## For debugging 
`helm template --dry-run --debug -name <release_name> <chart_path>`
```
helm template --dry-run --debug -name cronjobs spamoor-cronjobs
```


## Configuration

Via `values.yaml`

### Overview

```yaml
jobs:
  jobname-1:
    # job definition
  jobname-2:
    # job definition
  jobname-n:
    # job definition
```

### Details

```yaml
jobs:
  ### REQUIRED ###
  <job_name>:
    image:
      repository: <image_repo>
      tag: <image_tag>
      imagePullPolicy: <pull_policy>
    schedule: "<cron_schedule>"
    failedJobsHistoryLimit: <failed_history_limit>
    successfulJobsHistoryLimit: <successful_history_limit>
    concurrencyPolicy: <concurrency_policy>
    restartPolicy: <restart_policy>
  ### OPTIONAL ###
    imagePullSecrets:
    - username: <user>
      password: <password>
      email: <email>
      registry: <registry>
    env:
    - name: ENV_VAR
      value: ENV_VALUE
    envFrom:
    - secretRef:
      name: <secret_name>
    - configMapRef:
      name: <configmap_name>
    command: ["<command>"]
    args:
    - "<arg_1>"
    - "<arg_2>"
    resources:
      limits:
        cpu: <cpu_count>
        memory: <memory_count>
      requests:
        cpu: <cpu_count>
        memory: <memory_count>
    serviceAccount:
      name: <account_name>
      annotations:  # Optional
        my-annotation-1: <value>
        my-annotation-2: <value>
    nodeSelector:
      key: <value>
    tolerations:
    - effect: NoSchedule
      operator: Exists
    volumes:
      - name: config-mount
        configMap:
          name: configmap-name
          items:
            - key: configuration.yml
              path: configuration.yml
    volumeMounts:
      - name: config-mount
        mountPath: /etc/config
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/e2e-az-name
              operator: In
              values:
              - e2e-az1
              - e2e-az2
```

## Examples
```
$ helm install test-cron-job .
NAME: test-cron-job
LAST DEPLOYED: Tue Jun  4 12:27:32 2024
NAMESPACE: spamoor-cronjobs
STATUS: deployed
REVISION: 1
TEST SUITE: None

RESOURCES:
==> v1/CronJob
NAME                    AGE
test-cronjob-curl    1s
```
list cronjobs:
```
$ kubectl get cronjob
NAME                 SCHEDULE    SUSPEND   ACTIVE   LAST SCHEDULE   AGE
test-cron-job-curl   * * * * *   False     1        5m23s           5m49s
```
list jobs:
```
$ kubectl get jobs
NAME                                DESIRED   SUCCESSFUL   AGE
test-cron-job-curl-28625488   0/1           6m6s       6m6s
```
