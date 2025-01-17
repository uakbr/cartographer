# Spec Reference

## GVK

### Version

All of the custom resources that Cartographer is working on are being written under `v1alpha1` to indicate that our first version of it is at the "alpha stability level", and that it's our first iteration on it.

See [versions in CustomResourceDefinitions].

[versions in CustomResourceDefinitions]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/


### Group

All of our custom resources under the `carto.run` group[^1].

For instance:

```yaml
apiVersion: carto.run/v1alpha1
kind: ClusterSupplyChain
```

## Resources

Cartographer is composed of several custom resources, some of them being cluster-wide:

- [`ClusterSupplyChain`](#clustersupplychain)
- [`ClusterSourceTemplate`](#clustersourcetemplate)
- [`ClusterImageTemplate`](#clusterimagetemplate)
- [`ClusterConfigTemplate`](#clusterconfigtemplate)
- [`ClusterTemplate`](#clustertemplate)
- [`ClusterRunTemplate`](#clusterruntemplate)

and a few namespace-scoped:

- [`Workload`](#workload)
- [`Delivery`](#delivery)
- [`Runnable`](#runnable)


### Workload

`Workload` allows the developer to pass information about the app to be delivered through the supply chain.


```yaml
apiVersion: carto.run/v1alpha1
kind: Workload
metadata:
  name: spring-petclinic
  labels:
    # label to be matched against a `ClusterSupplyChain`s label selector.
    #
    app.tanzu.vmware.com/workload-type: web   # (1)

spec:
  source:
    # source code location in a git repository.
    #
    git:
      url: https://github.com/scothis/spring-petclinic.git
      ref:
        branch: "main"
        tag: "v0.0.1"
        commit: "b4df00d"

    # image containing the source code to be used throughout
    # the supply chain
    #
    image: harbor-repo.vmware.com/tanzu_desktop/golang-sample-source@sha256:e508a587

  build:
    # environment variables to propagate to a resource responsible
    # for performing a build in the supplychain.
    #
    env:
      - name: CGO_ENABLED
        value: "0"


  # serviceClaims to be bound through service-bindings
  #
  serviceClaims:
    - name: broker
      ref:
        apiVersion: services.tanzu.vmware.com/v1alpha1
        kind: RabbitMQ
        name: rabbit-broker


  # image with the app already built
  #
  image: foo/docker-built@sha256:b4df00d      # (2)

  # environment variables to be passed to the main container
  # running the application.
  #
  env:
    - name: SPRING_PROFILES_ACTIVE
      value: mysql


  # resource constraints for the main application.
  #
  resources:
    requests:
      memory: 1Gi
      cpu: 100m
    limits:
      memory: 1Gi
      cpu: 4000m

  # any other parameters that don't fit the ones already typed.
  params:
    - name: java-version
      # name of the parameter. should match a supply chain parameter name
      value: 11
```

notes:

1. labels serve as a way of indirectly selecting `ClusterSupplyChain` - `Workload`s without labels that match a `ClusterSupplyChain`'s `spec.selector` won't be reconciled and will stay in an `Errored` state.

2. `spec.image` is useful for enabling workflows that are not based on building the container image from within the supplychain, but outside.

_ref: [pkg/apis/v1alpha1/workload.go](../../../pkg/apis/v1alpha1/workload.go)_


### ClusterSupplyChain

With a `ClusterSupplyChain`, app operators describe which "shape of applications" they deal with (via `spec.selector`), and what series of resources are responsible for creating an artifact that delivers it (via `spec.resources`).

Those `Workload`s that match `spec.selector` then go through the resources specified in `spec.resources`.

A resource can emit values, which the supply chain can make available to other resources.

```yaml
apiVersion: carto.run/v1alpha1
kind: ClusterSupplyChain
metadata:
  name: supplychain
spec:

  # specifies the label key-value pair to select workloads. (required, one one)
  #
  selector:
    app.tanzu.vmware.com/workload-type: web

  # parameters to override the defaults from the templates.
  # if a resource in the supply-chain specifies a parameter
  # of the same name that resource parameter clobber what is
  # specified here at the top level (this includes specification
  # as `value` vs `default`)
  #
  # in a template, these can be consumed as:
  #
  #   $(params.<name>)
  #
  # (optional)
  params:
    # name of the parameter. (required, unique in this list, and should match
    # a pre-defined parameter name in a template)
    #
    - name: java-version
      # value to be passed down to the template's parameters,  supporting
      # interpolation.
      #
      value: 6
      # when specified as `value`, a parameter of the same name on the workload will
      # be disregarded.
      #
    - name: jvm
      value: openjdk
      # when specified as `default`, a parameter of the same name on the workload will
      # overwrite this default value.
      #

  # set of resources that will take care of bringing the application to a
  # deliverable state. (required, at least 1)
  #
  resources:
    # name of the resource to be referenced by further resources in the chain.
    # (required, unique)
    #
    - name: source-provider
      # object reference to a template object that instructs how to
      # instantiate and keep the resource up to date. (required)
      #
      templateRef:
        kind: ClusterSourceTemplate
        name: git-repository-battery

    - name: built-image-provider
      templateRef:
        kind: ClusterImageTemplate
        name: kpack-battery

      # a set of resources that provide source information, that is, url and
      # revision.
      #
      # in a template, these can be consumed as:
      #
      #    $(sources.<name>.url)$
      #    $(sources.<name>.revision)$
      #
      # if there is only one source, it can be consumed as:
      #
      #    $(source.url)$
      #    $(sources.revision)$
      #
      # (optional)
      sources:
        # name of the resource to provide the source information. (required)
        #
        - resource: source-provider
          # name to be referenced in the template via a query over the list of
          # sources (for instance, `$(sources.provider.url)`.
          #
          # (required, unique in this list)
          #
          name: provider

      # (optional) set of resources that provide image information.
      #
      # in a template, these can be consumed as:
      #
      #   $(images.<name>.image)
      #
      # if there is only one image, it can be consumed as:
      #
      #   $(image)
      #
      images: []

      # (optional) set of resources that provide kubernetes configuration,
      # for instance, podTemplateSpecs.
      # in a template, these can be consumed as:
      #
      #   $(configs.<name>.config)
      #
      # if there is only one config, it can be consumed as:
      #
      #   $(config)
      #
      configs: []

      # parameters to override the defaults from the templates.
      # resource parameters override any parameter of the same name set
      # for the overall supply-chain in spec.params
      # in a template, these can be consumed as:
      #
      #   $(params.<name>)
      #
      # (optional)
      params:
        # name of the parameter. (required, unique in this list, and should match
        # template's pre-defined set of parameters)
        #
        - name: java-version
          # value to be passed down to the template's parameters,  supporting
          # interpolation.
          #
          default: 9
          # when specified as `default`, a parameter of the same name on the workload will
          # overwrite this default value.
          #
        - name: jvm
          value: openjdk
          # when specified as `value`, a parameter of the same name on the workload will
          # be disregarded
          #
```


_ref: [pkg/apis/v1alpha1/cluster_supply_chain.go](../../../pkg/apis/v1alpha1/cluster_supply_chain.go)_


### ClusterSourceTemplate

`ClusterSourceTemplate` indicates how the supply chain could instantiate an object responsible for providing source code.

The `ClusterSourceTemplate` requires definition of a `urlPath` and `revisionPath`. `ClusterSourceTemplate` will update its status to emit `url` and `revision` values, which are reflections of the values at the path on the created objects. The supply chain may make these values available to other resources.

```yaml
apiVersion: carto.run/v1alpha1
kind: ClusterSourceTemplate
metadata:
  name: git-repository-battery
spec:
  # default set of parameters. (optional)
  #
  params:
      # name of the parameter (required, unique in this list)
      #
    - name: git-implementation
      # default value if not specified in the resource that references
      # this templateClusterSupplyChain (required)
      #
      default: libgit2

  # jsonpath expression to instruct where in the object templated out source
  # code url information can be found. (required)
  #
  urlPath: .status.artifact.url

  # jsonpath expression to instruct where in the object templated out
  # source code revision information can be found. (required)
  #
  revisionPath: .status.artifact.revision

  # template for instantiating the source provider.
  #
  # data available for interpolation (`$(<json_path>)$`:
  #
  #     - workload  (access to the whole workload object)
  #     - params
  #     - sources   (if specified in the supply chain)
  #     - images    (if specified in the supply chain)
  #     - configs   (if specified in the supply chain)
  #
  # (required)
  #
  template:
    apiVersion: source.toolkit.fluxcd.io/v1beta1
    kind: GitRepository
    metadata:
      name: $(workload.metadata.name)$-source
    spec:
      interval: 3m
      url: $(workload.spec.source.git.url)$
      ref: $(workload.spec.source.git.ref)$
      gitImplementation: $(params.git-implementation.value)$
      ignore: ""
```

_ref: [pkg/apis/v1alpha1/cluster_source_template.go](../../../pkg/apis/v1alpha1/cluster_source_template.go)_


### ClusterImageTemplate

`ClusterImageTemplate` instructs how the supply chain should instantiate an object responsible for supplying container images, for instance, one that takes source code, builds a container image out of it.

The `ClusterImageTemplate` requires definition of an `imagePath`. `ClusterImageTemplate` will update its status to emit an `image` value, which is a reflection of the value at the path on the created object. The supply chain may make this value available to other resources.

```yaml
apiVersion: carto.run/v1alpha1
kind: ClusterImageTemplate
metadata:
  name: kpack-battery
spec:
  # default set of parameters. see ClusterSourceTemplate for more
  # information. (optional)
  #
  params: []

  # jsonpath expression to instruct where in the object templated out container
  # image information can be found. (required)
  #
  imagePath: .status.latestImage

  # template for instantiating the image provider.
  # same data available for interpolation as any other `*Template`. (required)
  #
  template:
    apiVersion: kpack.io/v1alpha2
    kind: Image
    metadata:
      name: $(workload.metadata.name)$-image
    spec:
      tag: projectcartographer/demo/$(workload.metadata.name)$
      serviceAccount: service-account
      builder:
        kind: ClusterBuilder
        name: java-builder
      source:
        blob:
          url: $(sources.provider.url)$
```

_ref: [pkg/apis/v1alpha1/cluster_image_template.go](../../../pkg/apis/v1alpha1/cluster_image_template.go)_


### ClusterConfigTemplate

Instructs the supply chain how to instantiate a Kubernetes object that knows how to make Kubernetes configurations available to further resources in the chain.

The `ClusterConfigTemplate` requires definition of a `configPath`. `ClusterConfigTemplate` will update its status to emit a `config` value, which is a reflection of the value at the path on the created object. The supply chain may make this value available to other resources.

_ref: [pkg/apis/v1alpha1/cluster_config_template.go](../../../pkg/apis/v1alpha1/cluster_config_template.go)_


### ClusterTemplate

A `ClusterTemplate` instructs the supply chain to instantiate a Kubernetes object that has no outputs to be supplied to other objects in the chain, for instance, a resource that deploys a container image that has been built by other ancestor resources.

The `ClusterTemplate` does not emit values to the supply chain.

```yaml
apiVersion: carto.run/v1alpha1
kind: ConfigTemplate
metadata:
  name: deployer
spec:
  # default parameters. see ClusterSourceTemplate for more info. (optional)
  #
  params: []

  # how to template out the kubernetes object. (required)
  #
  template:
    apiVersion: kappctrl.k14s.io/v1alpha1
    kind: App
    metadata:
      name: $(workload.metadata.name)
    spec:
      serviceAccountName: service-account
      fetch:
        - inline:
            paths:
              manifest.yml: |
                ---
                apiVersion: kapp.k14s.io/v1alpha1
                kind: Config
                rebaseRules:
                  - path: [metadata, annotations, serving.knative.dev/creator]
                    type: copy
                    sources: [new, existing]
                    resourceMatchers: &matchers
                      - apiVersionKindMatcher: {apiVersion: serving.knative.dev/v1, kind: Service}
                  - path: [metadata, annotations, serving.knative.dev/lastModifier]
                    type: copy
                    sources: [new, existing]
                    resourceMatchers: *matchers
                ---
                apiVersion: serving.knative.dev/v1
                kind: Service
                metadata:
                  name: links
                  labels:
                    app.kubernetes.io/part-of: $(workload.metadata.labels['app\.kubernetes\.io/part-of'])$
                spec:
                  template:
                    spec:
                      containers:
                        - image: $(images.<name-of-image-provider>.image)$
                          securityContext:
                            runAsUser: 1000
      template:
        - ytt: {}
      deploy:
        - kapp: {}
```

_ref: [pkg/apis/v1alpha1/cluster_template.go](../../../pkg/apis/v1alpha1/cluster_template.go)_


### Delivery

This section is [pending work from issue #286](https://github.com/vmware-tanzu/cartographer/issues/286)

### Runnable

A `Runnable` object declares the intention of having immutable objects
submitted to Kubernetes according to a template (via ClusterRunTemplate)
whenever any of the inputs passed to it changes. i.e., it allows us to provide
a mutable spec that drives the creation of immutable objects whenever that spec
changes.


```yaml
apiVersion: carto.run/v1alpha1
kind: Runnable
metadata:
  name: test-runner
spec:
  # data to be made available to the template of ClusterRunTemplate
  # that we point at.
  #
  # this field takes as value an object that maps strings to values of any
  # kind, which can then be reference in a template using jsonpath such as
  # `$(runnable.spec.inputs.<key...>)$`.
  #
  # (required)
  #
  inputs:
    serviceAccount: bla
    params:
       - name: foo
         value: bar


  # reference to a ClusterRunTemplate that defines how objects should be
  # created referencing the data passed to the Runnable.
  #
  # (required)
  #
  runTemplateRef:
    name: job-runner


  # an optional selection rule for finding an object that should be used
  # together with the one being stamped out by the runnable.
  #
  # an object found using the rules described here are made available during
  # interpolation time via `$(selected.<...object>)$`.
  #
  # (optional)
  #
  selector:
    resource:
      kind: Pipeline
      apiVersion: tekton.dev/v1beta1
    matchingLabels:
      pipelines.foo.bar: testing
```


### ClusterRunTemplate

A `ClusterRunTemplate` defines how an immutable object should be stamped out
based on data provided by a `Runnable`.

```yaml
apiVersion: carto.run/v1alpha1
kind: ClusterRunTemplate
metadata:
  name: image-builder
spec:
  # data to be gathered from the objects that it interpolates once they #
  # succeeded (based on the object presenting a condition with type 'Succeeded'
  # and status `True`).
  #
  # (optional)
  #
  outputs:
    # e.g., make available under the Runnable `outputs` section in `status` a
    # field called "latestImage" that exposes the result named 'IMAGE-DIGEST'
    # of a tekton task that builds a container image.
    #
    latestImage: .status.results[?(@.name=="IMAGE-DIGEST")].value


  # definition of the object to interpolate and submit to kubernetes.
  #
  # data available for interpolation:
  #   - `runnable`: the Runnable object that referenced this template.
  #
  #                 e.g.:  params:
  #                        - name: revison
  #                          value: $(runnable.spec.inputs.git-revision)$
  #
  #
  #   - `selected`: a related object that got selected using the Runnable
  #                 selector query.
  #
  #                 e.g.:  taskRef:
  #                          name: $(selected.metadata.name)$
  #                          namespace: $(selected.metadata.namespace)$
  #
  # (required)
  #
  template:
    apiVersion: tekton.dev/v1beta1
    kind: TaskRun
    metadata:
      generateName: $(runnable.metadata.name)$-
    spec:
      serviceAccountName: $(runnable.spec.inputs.serviceAccount)$
      taskRef: $(runnable.spec.inputs.taskRef)$
      params: $(runnable.spec.inputs.params)$
```

It differs from supply chain templates in some aspects:

- it cannot be referenced directly by a ClusterSupplyChain object (it can only
  be reference by a Runnable)

- `outputs` provide a free-form way of exposing any form of results from what
  has been run (i.e., submitted by the Runnable) to the status of the Runnable
  object (as opposed to typed "source", "image", and "config" from supply
  chains)

- templating context (values provided to the interpolation) is specific to the
  Runnable: the runnable object itself and the object resulting from the
  selection query.

- templated object metadata.name should not be set. differently from
  ClusterSupplyChain, a Runnable has the semantics of creating new objects on
  change, rather than patching. This means that on every input set change, a new
  name must be derived. To be sure that a name can always be generated,
  `metadata.generateName` should be set rather than `metadata.name`.

Similarly to other templates, it has a `template` field where data is taken (in
this case, from Runnable and selected objects via `runnable.spec.selector`) and
via `$()$` allows one to interpolate such data to form a final object.
