# Using a Liberty Runtime Provided in the Stack Run Image

The Open Liberty buildpack can use an Open Liberty or WebShere Liberty runtime provided in the stack run image. This
allows you to use configurations and optimizations provided by either the [openliberty/open-liberty](https://hub.docker.com/r/openliberty/open-liberty)
or [ibmcom/websphere-liberty](https://hub.docker.com/r/ibmcom/websphere-liberty) UBI-based container images.

Note that a custom builder is also required to be able to use a custom stack.

## Creating the Stack Image

Either the [openliberty/open-liberty](https://hub.docker.com/r/openliberty/open-liberty) or [ibmcom/websphere-liberty](https://hub.docker.com/r/ibmcom/websphere-liberty)
container images can be used to provide the runtime used by the Open Liberty buildpack using one of the following templates.

### Open Liberty

```dockerfile
# RUN IMAGE
FROM openliberty/open-liberty:full-java11-openj9-ubi as run

ENV CNB_USER_ID=1001
ENV CNB_GROUP_ID=0
ENV CNB_STACK_ID="io.buildpacks.stacks.liberty"
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.liberty"

# Set environment variables used by the Open Liberty CNB.
ENV SERVICE_BINDING_ROOT=/platform/bindings
ENV BPI_LIBERTY_ROOT=/opt/ol
ENV BPI_LIBERTY_RUNTIME_ROOT=${BPI_LIBERTY_ROOT}/wlp
ENV PATH=${BPI_LIBERTY_ROOT}/helpers/runtime:${BPI_LIBERTY_RUNTIME_ROOT}/bin:${PATH}

# Set user and group (as declared in the base image)
USER ${CNB_USER_ID}

# This script will add the requested server configurations (optionally), apply any interim fixes (optionally) and populate caches to optimize runtime
RUN configure.sh

FROM registry.access.redhat.com/ubi8/ubi:8.5 as build

# BUILD IMAGE
ENV CNB_USER_ID=1001
ENV CNB_GROUP_ID=0
ENV CNB_STACK_ID="io.buildpacks.stacks.liberty"
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.liberty"

# Provides hint to the Open Liberty buildpack which version of Liberty is being used at build time
ENV BPI_LIBERTY_RUNTIME_ROOT=/opt/ol/wlp
RUN mkdir -p ${BPI_LIBERTY_RUNTIME_ROOT}

RUN useradd --uid ${CNB_USER_ID} --gid ${CNB_GROUP_ID} -m -s /bin/bash cnb

RUN yum -y install git wget jq && wget https://github.com/sclevine/yj/releases/download/v5.0.0/yj-linux -O /usr/local/bin/yj && chmod +x /usr/local/bin/yj

# Set user and group (as declared in the base image)
USER ${CNB_USER_ID}
```

### WebSphere Liberty

```dockerfile
# RUN IMAGE
FROM ibmcom/websphere-liberty:21.0.0.9-full-java11-openj9-ubi as run

ENV CNB_USER_ID=1001
ENV CNB_GROUP_ID=0
ENV CNB_STACK_ID="io.buildpacks.stacks.liberty"
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.liberty"

# Set environment variables used by the Open Liberty CNB.
ENV SERVICE_BINDING_ROOT=/platform/bindings
ENV BPI_LIBERTY_ROOT=/opt/ibm
ENV BPI_LIBERTY_RUNTIME_ROOT=${BPI_LIBERTY_ROOT}/wlp
ENV PATH=${BPI_LIBERTY_ROOT}/helpers/runtime:${BPI_LIBERTY_RUNTIME_ROOT}/bin:${PATH}

# Set user and group (as declared in the base image)
USER ${CNB_USER_ID}

# This script will add the requested server configurations (optionally), apply any interim fixes (optionally) and populate caches to optimize runtime
RUN configure.sh

FROM registry.access.redhat.com/ubi8/ubi:8.5 as build

# BUILD IMAGE
ENV CNB_USER_ID=1001
ENV CNB_GROUP_ID=0
ENV CNB_STACK_ID="io.buildpacks.stacks.liberty"
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.liberty"

# Provides hint to the Open Liberty buildpack which version of Liberty is being used at build time
ENV BPI_LIBERTY_RUNTIME_ROOT=/opt/ibm/wlp
RUN mkdir -p ${BPI_LIBERTY_RUNTIME_ROOT}

RUN useradd --uid ${CNB_USER_ID} --gid ${CNB_GROUP_ID} -m -s /bin/bash cnb

RUN yum -y install git wget jq && wget https://github.com/sclevine/yj/releases/download/v5.0.0/yj-linux -O /usr/local/bin/yj && chmod +x /usr/local/bin/yj

# Set user and group (as declared in the base image)
USER ${CNB_USER_ID}
```

### Installing Open Liberty or WebSphere Liberty iFixes

Place iFix jar files in a directory named `interim-fixes` in the directory containing your Dockerfile.

Add the following to your Dockerfile before the `RUN configure.sh`:
```console
COPY --chown=${CNB_USER_ID}:${CNB_GROUP_ID}  interim-fixes /opt/ol/fixes/
```

### Selecting a Different Java Runtime

The Java runtime can be configured by updating the tag in the `FROM` for the run image in the template above with the 
desired version. For example, if you require the Java 8 version of Open Liberty, update the `FROM` for the run image
to use `openliberty/open-liberty:full-java8-openj9-ubi`.

### Building the Stack Images

After preparing the `Dockerfile` for the stack, use the following commands to build the run and build images that will
be used:

```console
$ docker build -t <image-name>-run:latest --target run .
$ docker build -t <image-name>-build:latest --target build .
```

Replace `<image-name>` with whatever image name you would like to use.

## Creating the Custom Builder

Here is an example of a custom builder descriptor that can be used to build Liberty applications using the custom stack
images that you have prepared earlier.

```toml
[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/ca-certificates:3.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/bellsoft-liberica:9.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/syft:1.3.0"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/leiningen:4.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/gradle:6.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/maven:6.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/liberty:0.4.0"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/procfile:5.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/environment-variables:4.0.1"

[[buildpacks]]
  uri = "docker://gcr.io/paketo-buildpacks/image-labels:4.0.1"

[[order]]

  [[order.group]]
    id = "paketo-buildpacks/ca-certificates"
    optional = true
    version = "3.0.1"

  [[order.group]]
    id = "paketo-buildpacks/bellsoft-liberica"
    version = "9.0.1"

  [[order.group]]
    id = "paketo-buildpacks/syft"
    optional = true
    version = "1.3.0"

  [[order.group]]
    id = "paketo-buildpacks/gradle"
    optional = true
    version = "6.0.1"

  [[order.group]]
    id = "paketo-buildpacks/maven"
    optional = true
    version = "6.0.1"

  [[order.group]]
    id = "paketo-buildpacks/liberty"
    optional = true

  [[order.group]]
    id = "paketo-buildpacks/procfile"
    optional = true
    version = "5.0.1"

  [[order.group]]
    id = "paketo-buildpacks/environment-variables"
    optional = true
    version = "4.0.1"

  [[order.group]]
    id = "paketo-buildpacks/image-labels"
    optional = true
    version = "4.0.1"

[stack]
  id = "io.buildpacks.stacks.liberty"
  run-image = "<image-name>-run:latest"
  build-image = "<image-name>-build:latest"
```

Replace the `stack.run-image` and `stack.build-image` values with the `image-name` value used during the build. The
builder can then be built by running:

```console
$ pack -v builder create mybuilder:latest --config <path-to-your-builder-desciptor--toml>
```

## Deploying a Liberty Application

With the stack images and custom builder created, a Liberty application can now be deployed using:

```console
$ pack build myapp --builder mybuilder:latest --env BP_LIBERTY_INSTALL_TYPE="none"
```
