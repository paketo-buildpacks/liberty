# Building Open Liberty with Tekton

This guide demonstrates how to build your Open Liberty application with the Cloud Native Buildpacks Tekton task. We
assume you already have Tekton other prerequisites outlined below installed.

## Prerequisites

* Kubernetes / Openshift cluster
* Tekton
* Tekton Triggers (optional)
* `Buildpacks` and `git-clone` Tekton tasks as outlined in [Buildpacks Tekton](https://buildpacks.io/docs/tools/tekton/) documentation
* Builder with the Open Liberty buildpack (e.g., [Creating the Custom Builder](https://github.com/paketo-buildpacks/open-liberty/blob/main/docs/using-liberty-stack.md#creating-the-custom-builder))

## Defining and Applying the Tekton Pipeline Resources

### Create PVC for Buildpack

Request storage for the buildpacks task by creating a PVC resource named `buildpacks-source-pvc`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: buildpacks-source-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

### Adding Credentials for Container Registry

Create a `Secret` containing your credentials that the build should use to authenticate to the container registry:

```console
$ kubectl create secret docker-registry docker-user-pass \
    --docker-username=<USERNAME> \
    --docker-password=<PASSWORD> \
    --docker-server=<LINK TO REGISTRY, e.g. https://index.docker.io/v1/ > \
    --namespace <namespace>
```

and then create a `ServiceAccount` that uses the secret:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: buildpacks-service-account
secrets:
  - name: docker-user-pass
```

### Creating the Pipeline Tekton Resource

Create the pipeline resource with the following YAML. Replace the value for `BUILDER_IMAGE` with the builder with the
Open Liberty buildpack.

```yaml
---
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: buildpacks-test-pipeline
spec:
  params:
    - name: url
      type: string
      description: repo to build
    - name: revision
      type: string
      description: revision to checkout
      default: ""
    - name: image
      type: string
      description: image URL to push
  workspaces:
    - name: source-workspace # Directory where application source is located. (REQUIRED)
    - name: cache-workspace # Directory where cache is stored (OPTIONAL)
  tasks:
    - name: fetch-repository # This task fetches a repository from github, using the `git-clone` task you installed
      taskRef:
        name: git-clone
      workspaces:
        - name: output
          workspace: source-workspace
      params:
        - name: url
          value: "$(params.url)"
        - name: revision
          value: "$(params.revision)"
        - name: subdirectory
          value: ""
        - name: deleteExisting
          value: "true"
    - name: buildpacks # This task uses the `buildpacks` task to build the application
      taskRef:
        name: buildpacks
      runAfter:
        - fetch-repository
      workspaces:
        - name: source
          workspace: source-workspace
        - name: cache
          workspace: cache-workspace
      params:
        - name: APP_IMAGE
          value: "$(params.image)"
        - name: BUILDER_IMAGE
          value: <builder-image>:<tag> # REPLACE THIS with the Open Liberty builder we want the task to use (REQUIRED)
    - name: display-results
      runAfter:
        - buildpacks
      taskSpec:
        steps:
          - name: print
            image: docker.io/library/bash:5.1.4@sha256:b208215a4655538be652b2769d82e576bc4d0a2bb132144c060efc5be8c3f5d6
            script: |
              #!/usr/bin/env bash
              set -e
              echo "Digest of created app image: $(params.DIGEST)"
        params:
          - name: DIGEST
      params:
        - name: DIGEST
          value: $(tasks.buildpacks.results.APP_IMAGE_DIGEST)
```

## Testing the Pipeline

Once you have created all the above resources, we can test the pipeline manually by creating a `PipelineRun`. Update the
`url` and `image` values to point to your git repository and container registry, respectively. Afterwards, apply the
`PipelineRun` resource to start the build:

```yaml
---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: test-pipeline-run
spec:
  serviceAccountName: buildpacks-service-account
  pipelineRef:
    name: buildpacks-test-pipeline
  workspaces:
    - name: source-workspace
      subPath: source
      persistentVolumeClaim:
        claimName: buildpacks-source-pvc
    - name: cache-workspace
      subPath: cache
      persistentVolumeClaim:
        claimName: buildpacks-source-pvc
  params:
    - name: url
      value: https://github.com/<org>/<repository>
    - name: image
      value: <container-registry>/<image>[:tag]
```

You can check the results of the pipeline run using `tkn pr list` or `kubectl get pipelineruns`.

## Trigger Builds for GitHub Repositories

`PipelineRuns` can be automatically created by GitHub pull requests and pushes using `Tekton Triggers`. If you do not
have Tekton Triggers installed, install using [these](https://github.com/tektoncd/triggers/blob/main/docs/install.md)
directions.

The Tekton Trigger resources can be used with minor modifications if using the `Pipeline` we defined above.

### Tekton Trigger Bindings

The `TriggerBinding` resource extracts information from an event payload and binds them to the specified parameter
names. For example, the following binding extracts the pull request commit SHA and repository URL from a GitHub
`PullRequestEvent` and binds them to the parameters `gitrevision`, and `gitrepositoryurl`, respectively.

```yaml
---
apiVersion: triggers.tekton.dev/v1beta1
kind: TriggerBinding
metadata:
  name: buildpacks-pipeline-binding
spec:
  params:
    - name: gitrevision
      value: $(body.pull_request.head.sha)
    - name: gitrepositoryurl
      value: $(body.repository.html_url)
    - name: pull_request_title
      value: $(body.pull_request.title)

```

### Tekton Trigger Templates

The `TriggerTemplate` takes the parameters bound by the `TriggerBinding` and passes them to the pipeline
`buildpacks-test-pipeline` that we defined previously. Update the value for the `image` parameter to container registry
and image that you would like to push the image to.

```yaml
---
apiVersion: triggers.tekton.dev/v1beta1
kind: TriggerTemplate
metadata:
  name: buildpacks-trigger-pipeline-template
spec:
  params:
    - name: gitrevision
    - name: gitrepositoryurl
    - name: pull_request_title
  resourceTemplates:
  - apiVersion: tekton.dev/v1beta1
    kind: PipelineRun
    metadata:
      generateName: buildpacks-app-run-pr-tr-
    spec:
      serviceAccountName: buildpacks-service-account
      pipelineRef:
        name: buildpacks-test-pipeline
      workspaces:
        - name: source-workspace
          subPath: source
          persistentVolumeClaim:
            claimName: buildpacks-source-pvc
        - name: cache-workspace
          subPath: cache
          persistentVolumeClaim:
            claimName: buildpacks-source-pvc
      params:
      - name: url
        value: $(tt.params.gitrepositoryurl)
      - name: revision
        value: $(tt.params.gitrevision)
      - name: image
        value: <container-registry>/<image>[:tag]
```

### Tekton EventListener

The `EventListener` creates a listener for events and specify triggers that you'd like to create `PipelineRuns` for. The
example `EventListener` creates a `GitHub` event listener that triggers a `PipelineRun` for pull requests using the
`TriggerTemplate` and `TriggerBinding` resources defined above.

Update the `serviceAccountName` with the Tekton service account needed as described
[here](https://github.com/tektoncd/triggers/blob/main/docs/eventlisteners.md#specifying-the-kubernetes-service-account)
directions. If you are using OpenShift and OpenShift Pipeline operator, you can use the `pipeline` service account.

```yaml
---
apiVersion: triggers.tekton.dev/v1beta1
kind: EventListener
metadata:
  name: github-event-listener
spec:
  serviceAccountName: <tekton-service-account>
  triggers:
    - name: github-listener
      bindings:
        - ref: buildpacks-pipeline-binding
      template:
        ref: buildpacks-trigger-pipeline-template
```

After applying the `EventListener` resource, get the route for the event listener and create GitHub webhook that points
to it in your project's repository.
