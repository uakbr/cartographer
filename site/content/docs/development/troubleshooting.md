# Troubleshooting

## Reading your workload or deliverable status

Cartographer makes every effort to provide you with useful information in the `status` field of your `workload`
or `deliverable` object.

To see the status of your workload:

```shell
kubectl get workload <your-workload-name> -n <your-workload-namespace> -oyaml
```

Note: We do not recommend `kubectl describe` as it makes statuses harder to read.

Take a look at the `status:` section for conditions.

## Common workload and deliverable conditions

Cartographer conditions follow the [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) 
There is a top level condition of `Type: Ready` which will have `Status: Unknown` or `Status: False` to match a subcondition.

If your workload or deliverable has a `False` or a `Unknown` condition, inspect the conditions for cause.
The `Type: Ready` condition's `Reason` will match that of the subcondition causing the negative status.

### Unknown vs. False

A status of `False` typically means Cartographer can not proceed until user intervention occurs. These are errors in 
configuration.

A status of `Unknown` indicates that resources have not yet resolved. Causes can include network timeouts, long running
processes, and occasionally a misconfiguration that Cartographer cannot itself detect.

### MissingValueAtPath

This is the most common `Unknown` state.
 
```text
status:
  conditions:
    - type: SupplyChainReady
      status: True
      reason: Ready
    - type: ResourcesSubmitted
      status: Unknown
      reason: MissingValueAtPath
      message: Waiting to read value [.status.artifact.revision] from resource [images.kpack.io/cool-app] in namespace [default]
    - type: Ready
      status: Unknown
      reason: MissingValueAtPath
```

You will see this as part of normal operation, however if your `workload` or `deliverable` are taking a long time to
become ready, then there might 
## Viewing your supply chain or delivery instances
